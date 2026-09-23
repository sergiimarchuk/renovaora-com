#!/bin/bash
# ============================================================
# disable_cloudinit_network.sh — отключить сетевой менеджмент
#                                 cloud-init на нодах кластера
#
# Зачем: пока cloud-init управляет сетью, любой cloud-init clean
# (или, в некоторых случаях, holy cold boot / recreate ISO на
# Proxmox) заново применит СТАРЫЕ сетевые параметры, зашитые в
# qm config на момент создания VM — перезаписав любые ручные
# правки (смена IP на DHCP/статику, другой шлюз, другая подсеть
# и т.д.), сделанные позже внутри гостя.
#
# Этот скрипт один раз создаёт на каждой ноде файл, который
# говорит cloud-init больше не трогать сеть вообще — дальше сеть
# полностью под контролем netconfig/wicked (или NetworkManager)
# внутри самой ноды, и cloud-init clean на неё не влияет.
#
# Использование:
#   ./disable_cloudinit_network.sh [config.json]
# По умолчанию читает IP нод из k8s_ha_vms_config.json,
# либо можно передать список IP через переменную окружения:
#   NODES="192.168.0.150 192.168.0.151 192.168.0.152" ./disable_cloudinit_network.sh
# ============================================================

set -e

CONFIG="${1:-k8s_ha_vms_config.json}"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; NC='\033[0m'
log()  { echo -e "${GREEN}[✓]${NC} $1"; }
info() { echo -e "${BLUE}[→]${NC} $1"; }
error(){ echo -e "${RED}[✗]${NC} $1"; exit 1; }

if [ -n "$NODES" ]; then
    read -ra IP_LIST <<< "$NODES"
else
    command -v jq &>/dev/null || error "jq не установлен, либо задай NODES=\"ip1 ip2 ip3\" вручную"
    [ -f "$CONFIG" ] || error "Конфиг не найден: $CONFIG (или задай NODES=\"ip1 ip2 ip3\")"
    mapfile -t IP_LIST < <(jq -r '.vms[].ip' "$CONFIG")
fi

echo "=== Отключение cloud-init network management ==="
echo "  Ноды: ${IP_LIST[*]}"
echo ""

for IP in "${IP_LIST[@]}"; do
    info "Обрабатываю $IP ..."
    ssh $SSH_OPTS root@"$IP" bash <<'ENDSSH'
set -e
mkdir -p /etc/cloud/cloud.cfg.d
cat > /etc/cloud/cloud.cfg.d/99-disable-network-config.cfg <<'EOF'
network: {config: disabled}
EOF
echo "  Файл создан:"
cat /etc/cloud/cloud.cfg.d/99-disable-network-config.cfg
ENDSSH
    log "$IP — cloud-init network management отключен"
done

echo ""
log "Готово на всех нодах."
echo ""
echo "Проверка (не обязательно, но полезно перед реальными сетевыми правками):"
echo "  ssh root@<нода> \"cloud-init clean --logs && netconfig update -f && cat /etc/resolv.conf\""
echo "  — если сеть не откатилась к старым значениям, всё работает как надо."
echo ""
echo "После этого сеть можно менять как угодно (DHCP, статика, другой шлюз/подсеть)"
echo "напрямую внутри гостя (netconfig/wicked, NetworkManager) — cloud-init"
echo "больше не будет её перезаписывать при ребуте или cloud-init clean."
