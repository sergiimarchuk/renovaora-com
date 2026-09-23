#!/bin/bash
# ============================================================
# k8_deploy_ha_cluster.sh — HA Kubernetes cluster (3 control-plane
#                            nodes, stacked etcd, keepalived+HAProxy VIP)
# OS нод: openSUSE Leap 15.6
#
# Использование: ./k8_deploy_ha_cluster.sh [config.json]
# По умолчанию:  ./k8_deploy_ha_cluster.sh k8s_ha_vms_config.json
#
# Требует: VMs уже созданы (см. create_vms_from_config.sh) и
# доступны по SSH, с ключом root@ ~/.ssh/id_rsa.pub уже добавленным.
# ============================================================

set -e

CONFIG="${1:-k8s_ha_vms_config.json}"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"
K8S_VERSION="1.34"   # <-- check https://kubernetes.io/releases/ before running; only the latest 3 minors are supported upstream

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()     { echo -e "${GREEN}[✓]${NC} $1"; }
info()    { echo -e "${BLUE}[→]${NC} $1"; }
warn()    { echo -e "${YELLOW}[!]${NC} $1"; }
error()   { echo -e "${RED}[✗]${NC} $1"; exit 1; }
section() { echo ""; echo -e "${BLUE}══════════════════════════════════════${NC}"; echo -e "${BLUE}  $1${NC}"; echo -e "${BLUE}══════════════════════════════════════${NC}"; }

command -v jq &>/dev/null || error "jq не установлен: apt install jq"
[ -f "$CONFIG" ]          || error "Конфиг не найден: $CONFIG"
[ -f ~/.ssh/id_rsa.pub ]  || error "SSH ключ не найден ~/.ssh/id_rsa.pub"

VM_COUNT=$(jq '.vms | length' "$CONFIG")
[ "$VM_COUNT" -ge 3 ] || error "Для HA нужно минимум 3 ноды в конфиге, найдено: $VM_COUNT"

VIP=$(jq -r '.defaults.vip' "$CONFIG")
LB_PORT=$(jq -r '.defaults.lb_port' "$CONFIG")
MASTER_IP=$(jq -r '.vms[0].ip' "$CONFIG")
MASTER_NAME=$(jq -r '.vms[0].name' "$CONFIG")

section "HA Kubernetes кластер — openSUSE Leap 15.6"
echo "  Конфиг: $CONFIG | Нод: $VM_COUNT | VIP: $VIP:$LB_PORT | K8s: v$K8S_VERSION"

# ══════════════════════════════════════════════════════════
# ЧАСТЬ 1: Проверка доступности нод
# ══════════════════════════════════════════════════════════
section "Часть 1: Проверка доступности нод"

ALL_IPS=()
for i in $(seq 0 $((VM_COUNT - 1))); do
    IP=$(jq -r ".vms[$i].ip" "$CONFIG")
    NAME=$(jq -r ".vms[$i].name" "$CONFIG")
    ALL_IPS+=("$IP")

    ssh-keygen -f ~/.ssh/known_hosts -R "$IP" &>/dev/null || true

    TIMEOUT=180; ELAPSED=0
    info "Ожидание SSH на $NAME ($IP)..."
    while [ $ELAPSED -lt $TIMEOUT ]; do
        if ssh $SSH_OPTS root@$IP "echo ok" &>/dev/null; then
            log "$NAME доступен"
            break
        fi
        sleep 5; ELAPSED=$((ELAPSED+5)); echo -n "."
    done
    echo ""
    [ $ELAPSED -ge $TIMEOUT ] && error "Timeout: $NAME ($IP) недоступен"
done

info "Проверка, что VIP $VIP свободен (не должен отвечать на ping)..."
if ping -c1 -W2 "$VIP" &>/dev/null; then
    error "VIP $VIP уже кем-то используется! Выбери другой адрес в конфиге (.defaults.vip)."
fi
log "VIP $VIP свободен"

# ══════════════════════════════════════════════════════════
# ЧАСТЬ 2: Подготовка нод (base packages + keepalived + haproxy)
# ══════════════════════════════════════════════════════════
section "Часть 2: Подготовка нод"

wait_zypper() {
    local IP=$1
    local TIMEOUT=30
    local ELAPSED=0
    warn "Ожидание освобождения zypper на $IP..."
    while [ $ELAPSED -lt $TIMEOUT ]; do
        BUSY=$(ssh $SSH_OPTS root@$IP "pgrep -x zypper || pgrep -x rpm || ls /var/run/zypp.pid 2>/dev/null | wc -l" 2>/dev/null || echo "0")
        if [ "$BUSY" = "0" ]; then
            log "zypper свободен"
            return 0
        fi
        echo -n "."; sleep 5; ELAPSED=$((ELAPSED+5))
    done
    echo ""
    warn "Принудительное завершение zypper на $IP..."
    ssh $SSH_OPTS root@$IP "pkill -9 zypper 2>/dev/null; pkill -9 rpm 2>/dev/null; rm -f /var/run/zypp.pid /run/zypp/zypp.pid /var/lib/rpm/.rpm.lock /var/lib/rpm/__db*" 2>/dev/null || true
    sleep 5
}

# Build the haproxy backend server list once (same on every node)
HAPROXY_BACKENDS=""
for i in $(seq 0 $((VM_COUNT - 1))); do
    IP=$(jq -r ".vms[$i].ip" "$CONFIG")
    NAME=$(jq -r ".vms[$i].name" "$CONFIG")
    HAPROXY_BACKENDS="${HAPROXY_BACKENDS}    server $NAME $IP:6443 check fall 3 rise 2\n"
done

prepare_node() {
    local IDX=$1
    local IP=$2
    local NAME=$3
    local PRIORITY=$4

    # Build this node's unicast_peer list (every other node's IP)
    local PEERS=""
    for j in $(seq 0 $((VM_COUNT - 1))); do
        [ "$j" = "$IDX" ] && continue
        PEER_IP=$(jq -r ".vms[$j].ip" "$CONFIG")
        PEERS="${PEERS}        $PEER_IP\n"
    done

    local STATE="BACKUP"
    [ "$IDX" = "0" ] && STATE="MASTER"

    info "Подготовка $NAME ($IP), keepalived priority=$PRIORITY state=$STATE ..."
    wait_zypper "$IP"

    ssh $SSH_OPTS root@$IP bash <<ENDSSH
set -e

echo "=== [0/10] Отключение сетевого менеджмента cloud-init ==="
# После первого применения cloud-init сети, останавливаем его дальнейшее
# вмешательство: если позже понадобится вручную сменить IP на DHCP, другой
# шлюз/подсеть и т.д. — cloud-init не будет откатывать изменения при
# следующей перезагрузке / cloud-init clean.
mkdir -p /etc/cloud/cloud.cfg.d
cat > /etc/cloud/cloud.cfg.d/99-disable-network-config.cfg <<EOF2
network: {config: disabled}
EOF2
echo "  cloud-init network management отключен — сеть теперь под ручным/netconfig контролем"

echo "=== [1/10] Chrony ==="
zypper --gpg-auto-import-keys install -y chrony
systemctl enable chronyd && systemctl start chronyd

echo "=== [2/10] Swap ==="
swapoff -a
sed -i '/swap/s/^/#/' /etc/fstab

echo "=== [3/10] Модули ядра ==="
cat > /etc/modules-load.d/k8s.conf <<EOF
overlay
br_netfilter
EOF
modprobe overlay
modprobe br_netfilter

echo "=== [4/10] sysctl ==="
cat > /etc/sysctl.d/k8s.conf <<EOF
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF
sysctl --system > /dev/null

echo "=== [5/10] Firewall ==="
zypper --gpg-auto-import-keys install -y firewalld
systemctl enable firewalld
systemctl start firewalld
firewall-cmd --set-default-zone=trusted

# All nodes are control-plane in this HA layout
firewall-cmd --permanent --add-port=6443/tcp
firewall-cmd --permanent --add-port=2379-2380/tcp
firewall-cmd --permanent --add-port=10250/tcp
firewall-cmd --permanent --add-port=10257/tcp
firewall-cmd --permanent --add-port=10259/tcp
firewall-cmd --permanent --add-port=8472/udp
firewall-cmd --permanent --add-port=${LB_PORT}/tcp
firewall-cmd --permanent --add-protocol=vrrp
firewall-cmd --set-default-zone=trusted
firewall-cmd --reload
echo "  Порты control-plane + LB ($LB_PORT/tcp) + VRRP открыты"

echo "=== [6/10] AppArmor ==="
if systemctl is-active --quiet apparmor 2>/dev/null; then
    [ -f /etc/apparmor.d/usr.sbin.containerd ] && aa-complain /etc/apparmor.d/usr.sbin.containerd 2>/dev/null || true
    [ -f /etc/apparmor.d/usr.bin.containerd ]  && aa-complain /etc/apparmor.d/usr.bin.containerd  2>/dev/null || true
    echo "  containerd -> complain mode"
else
    echo "  AppArmor не активен"
fi

echo "=== [7/10] Containerd + conntrack + ethtool ==="
zypper --gpg-auto-import-keys install -y containerd conntrack-tools ethtool
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sed -i 's|sandbox_image = ".*"|sandbox_image = "registry.k8s.io/pause:3.10"|' /etc/containerd/config.toml
systemctl restart containerd
systemctl enable containerd

echo "=== [8/10] kubeadm kubelet kubectl (v${K8S_VERSION}) ==="
cat > /etc/zypp/repos.d/kubernetes.repo <<EOF
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/rpm/repodata/repomd.xml.key
EOF
zypper --gpg-auto-import-keys refresh
zypper --gpg-auto-import-keys install -y kubelet kubeadm kubectl
systemctl enable kubelet
echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >> /root/.bashrc

echo "=== [9/10] keepalived + haproxy (VIP ${VIP}:${LB_PORT}) ==="
zypper --gpg-auto-import-keys install -y keepalived haproxy

IFACE=\$(ip -4 route show default | awk '{print \$5; exit}')
echo "  Using interface: \$IFACE"

cat > /etc/haproxy/haproxy.cfg <<EOF
global
    log /dev/log local0
    maxconn 4000

defaults
    log     global
    mode    tcp
    option  tcplog
    timeout connect 5s
    timeout client  50s
    timeout server  50s

frontend k8s-api-lb
    bind *:${LB_PORT}
    mode tcp
    default_backend k8s-api-backend

backend k8s-api-backend
    mode tcp
    balance roundrobin
    option tcp-check
$(echo -e "$HAPROXY_BACKENDS")
EOF

cat > /etc/keepalived/keepalived.conf <<EOF
vrrp_script chk_haproxy {
    script "killall -0 haproxy"
    interval 2
    weight -20
}

vrrp_instance VI_1 {
    state ${STATE}
    interface \$IFACE
    virtual_router_id 51
    priority ${PRIORITY}
    advert_int 1
    unicast_src_ip ${IP}
    unicast_peer {
$(echo -e "$PEERS")
    }
    authentication {
        auth_type PASS
        auth_pass k8sHaSecret
    }
    virtual_ipaddress {
        ${VIP}/24
    }
    track_script {
        chk_haproxy
    }
}
EOF

systemctl enable haproxy && systemctl restart haproxy
systemctl enable keepalived && systemctl restart keepalived
echo "  keepalived + haproxy настроены (state=${STATE}, priority=${PRIORITY})"

echo "✓ $NAME готова!"
ENDSSH

    log "$NAME подготовлена"
}

for i in $(seq 0 $((VM_COUNT - 1))); do
    IP=$(jq -r ".vms[$i].ip" "$CONFIG")
    NAME=$(jq -r ".vms[$i].name" "$CONFIG")
    PRIORITY=$((150 - i * 30))
    prepare_node "$i" "$IP" "$NAME" "$PRIORITY"
done

info "Проверка, что VIP поднялся (должен быть на первой ноде, $MASTER_NAME)..."
sleep 5
ssh $SSH_OPTS root@$MASTER_IP "ip -4 addr show | grep $VIP" && log "VIP $VIP активен на $MASTER_NAME" \
    || warn "VIP пока не появился — проверь 'systemctl status keepalived' на нодах вручную"

# ══════════════════════════════════════════════════════════
# ЧАСТЬ 3: Инициализация первого control-plane узла
# ══════════════════════════════════════════════════════════
section "Часть 3: Инициализация первого control-plane узла ($MASTER_NAME)"

info "kubeadm init (2-5 минут)..."
ssh $SSH_OPTS root@$MASTER_IP bash <<ENDSSH
set -e
kubeadm init \
  --control-plane-endpoint "${VIP}:${LB_PORT}" \
  --upload-certs \
  --pod-network-cidr=10.244.0.0/16 \
  --apiserver-advertise-address=$MASTER_IP
mkdir -p \$HOME/.kube
cp /etc/kubernetes/admin.conf \$HOME/.kube/config
chown \$(id -u):\$(id -g) \$HOME/.kube/config
ENDSSH
log "Первый control-plane узел инициализирован (endpoint: $VIP:$LB_PORT)"

info "Создание ClusterRoleBinding..."
ssh $SSH_OPTS root@$MASTER_IP bash <<ENDSSH
set -e
openssl genrsa -out /tmp/admin.key 2048
openssl req -new -key /tmp/admin.key -out /tmp/admin.csr -subj "/O=system:masters/CN=admin"
openssl x509 -req -in /tmp/admin.csr \
  -CA /etc/kubernetes/pki/ca.crt \
  -CAkey /etc/kubernetes/pki/ca.key \
  -CAcreateserial -out /tmp/admin.crt -days 365

curl -sk \
  --cert /tmp/admin.crt \
  --key /tmp/admin.key \
  -X POST \
  -H "Content-Type: application/json" \
  https://127.0.0.1:6443/apis/rbac.authorization.k8s.io/v1/clusterrolebindings \
  -d '{
    "apiVersion": "rbac.authorization.k8s.io/v1",
    "kind": "ClusterRoleBinding",
    "metadata": {"name": "kubeadm:cluster-admins"},
    "roleRef": {
      "apiGroup": "rbac.authorization.k8s.io",
      "kind": "ClusterRole",
      "name": "cluster-admin"
    },
    "subjects": [{
      "apiGroup": "rbac.authorization.k8s.io",
      "kind": "Group",
      "name": "kubeadm:cluster-admins"
    }]
  }' > /dev/null

rm -f /tmp/admin.key /tmp/admin.csr /tmp/admin.crt
echo "✓ ClusterRoleBinding создан"
ENDSSH
log "ClusterRoleBinding готов"

info "Установка Flannel..."
ssh $SSH_OPTS root@$MASTER_IP \
    "kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml"
log "Flannel установлен"

# ══════════════════════════════════════════════════════════
# ЧАСТЬ 4: Присоединение остальных control-plane узлов
# ══════════════════════════════════════════════════════════
section "Часть 4: Присоединение остальных control-plane узлов"

JOIN_CMD=$(ssh $SSH_OPTS root@$MASTER_IP "kubeadm token create --print-join-command")
CERT_KEY=$(ssh $SSH_OPTS root@$MASTER_IP "kubeadm init phase upload-certs --upload-certs 2>/dev/null | tail -1")
log "Join-команда и certificate-key получены (действительны 2 часа)"

for i in $(seq 1 $((VM_COUNT - 1))); do
    NODE_IP=$(jq -r ".vms[$i].ip" "$CONFIG")
    NODE_NAME=$(jq -r ".vms[$i].name" "$CONFIG")

    info "Присоединение $NODE_NAME ($NODE_IP) как control-plane..."
    ssh $SSH_OPTS root@$NODE_IP "$JOIN_CMD --control-plane --certificate-key $CERT_KEY"

    ssh $SSH_OPTS root@$NODE_IP bash <<ENDSSH
mkdir -p \$HOME/.kube
cp /etc/kubernetes/admin.conf \$HOME/.kube/config
chown \$(id -u):\$(id -g) \$HOME/.kube/config
ENDSSH
    log "$NODE_NAME присоединён"
done

# ══════════════════════════════════════════════════════════
# ЧАСТЬ 5: Снятие control-plane taint (нет выделенных воркеров)
# ══════════════════════════════════════════════════════════
section "Часть 5: Разрешаем поды на всех нодах (нет выделенных воркеров)"

ssh $SSH_OPTS root@$MASTER_IP "kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true"
log "Taint снят со всех нод"

# ══════════════════════════════════════════════════════════
section "✓ HA кластер готов!"
echo ""
echo "  VIP:          https://$VIP:$LB_PORT"
echo "  Control-plane нод: $VM_COUNT"
echo ""
echo "  Подожди 60 секунд и проверь:"
echo "  ssh root@$MASTER_IP 'kubectl get nodes -o wide'"
echo ""
echo "  Проверка etcd-кворума:"
echo "  ssh root@$MASTER_IP 'ETCDCTL_API=3 etcdctl member list --endpoints=https://127.0.0.1:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key -w table'"
echo ""
