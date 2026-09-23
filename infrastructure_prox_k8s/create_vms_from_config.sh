#!/bin/bash
# ============================================================
# create_vms_from_config.sh — Batch-create Proxmox VMs from a
#                              JSON config (openSUSE cloud-init)
#
# Usage: ./create_vms_from_config.sh [config.json]
# Default: ./create_vms_from_config.sh k8s_ha_vms_config.json
#
# Run this ON THE PROXMOX HOST (root@prox), from /opt/creation_vm.
# ============================================================

set -e

CONFIG="${1:-k8s_ha_vms_config.json}"

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[✓]${NC} $1"; }
info() { echo -e "${BLUE}[→]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error(){ echo -e "${RED}[✗]${NC} $1"; exit 1; }

command -v jq &>/dev/null || error "jq не установлен: apt install jq"
[ -f "$CONFIG" ] || error "Конфиг не найден: $CONFIG"

IMAGE_PATH=$(jq -r '.defaults.image_path' "$CONFIG")
GATEWAY=$(jq -r '.defaults.gateway' "$CONFIG")
NAMESERVER=$(jq -r '.defaults.nameserver' "$CONFIG")
SEARCHDOMAIN=$(jq -r '.defaults.searchdomain // empty' "$CONFIG")
STORAGE=$(jq -r '.defaults.storage' "$CONFIG")
BRIDGE=$(jq -r '.defaults.bridge' "$CONFIG")

[ -f "$IMAGE_PATH" ] || error "Cloud image не найден: $IMAGE_PATH"
[ -f ~/.ssh/id_rsa.pub ] || error "SSH ключ не найден ~/.ssh/id_rsa.pub"

VM_COUNT=$(jq '.vms | length' "$CONFIG")

echo "=== Batch VM creation from $CONFIG ==="
echo "  Gateway: $GATEWAY"
echo "  Nameserver: $NAMESERVER"
echo "  VMs to create: $VM_COUNT"
echo ""

for entry in $(jq -r '.vms[].password' "$CONFIG"); do
    if [ "$entry" = "CHANGE_ME_BEFORE_RUNNING" ]; then
        error "Найден дефолтный пароль CHANGE_ME_BEFORE_RUNNING в $CONFIG — замени все пароли перед запуском."
    fi
done

read -p "Создать $VM_COUNT VM согласно конфигу выше? (y/n): " CONFIRM
[ "$CONFIRM" = "y" ] || { warn "Отменено пользователем"; exit 0; }

for i in $(seq 0 $((VM_COUNT - 1))); do
    VMID=$(jq -r ".vms[$i].id" "$CONFIG")
    VMNAME=$(jq -r ".vms[$i].name" "$CONFIG")
    MEMORY=$(jq -r ".vms[$i].memory" "$CONFIG")
    CORES=$(jq -r ".vms[$i].cores" "$CONFIG")
    DISKSIZE=$(jq -r ".vms[$i].disk" "$CONFIG")
    IPADDR=$(jq -r ".vms[$i].ip" "$CONFIG")
    PASSWORD=$(jq -r ".vms[$i].password" "$CONFIG")
    START=$(jq -r ".vms[$i].start" "$CONFIG")

    echo ""
    info "Создаю VM $VMID ($VMNAME) — $IPADDR ..."

    if qm status "$VMID" &>/dev/null; then
        warn "VMID $VMID уже существует — пропускаю (удали вручную если нужно пересоздать: qm stop $VMID && qm destroy $VMID)"
        continue
    fi

    qm create "$VMID" \
      --name "$VMNAME" \
      --memory "$MEMORY" \
      --cores "$CORES" \
      --sockets 1 \
      --cpu host \
      --numa 0 \
      --ostype l26 \
      --scsihw virtio-scsi-single \
      --net0 "virtio,bridge=$BRIDGE,firewall=1" \
      --agent enabled=1

    qm importdisk "$VMID" "$IMAGE_PATH" "$STORAGE"
    qm set "$VMID" --scsi0 "$STORAGE:vm-$VMID-disk-0"
    qm set "$VMID" --boot order=scsi0
    qm set "$VMID" --ide2 "$STORAGE:cloudinit"
    qm set "$VMID" --serial0 socket --vga serial0

    qm set "$VMID" --ciuser root
    qm set "$VMID" --cipassword "$PASSWORD"
    qm set "$VMID" --ipconfig0 "ip=$IPADDR/24,gw=$GATEWAY"
    qm set "$VMID" --nameserver "$NAMESERVER"
    [ -n "$SEARCHDOMAIN" ] && qm set "$VMID" --searchdomain "$SEARCHDOMAIN"
    qm set "$VMID" --sshkeys ~/.ssh/id_rsa.pub

    CURRENT_SIZE=1
    if [ "$DISKSIZE" -gt "$CURRENT_SIZE" ]; then
        RESIZE=$((DISKSIZE - CURRENT_SIZE))
        qm resize "$VMID" scsi0 +"${RESIZE}"G
    fi

    log "VM $VMID ($VMNAME) создана"

    if [ "$START" = "true" ]; then
        qm start "$VMID"
        log "VM $VMID запущена"
    fi
done

echo ""
log "Готово. Подожди 30-60 секунд, затем проверь SSH-доступность:"
for i in $(seq 0 $((VM_COUNT - 1))); do
    IP=$(jq -r ".vms[$i].ip" "$CONFIG")
    NAME=$(jq -r ".vms[$i].name" "$CONFIG")
    echo "  ssh root@$IP   # $NAME"
done
echo ""
echo "После этого запусти: ./k8_deploy_ha_cluster.sh $CONFIG"
