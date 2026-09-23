#!/bin/bash
# ============================================================
# update_dns_records.sh — синхронизировать A/PTR записи в BIND
#                          (base-host-01) с текущим списком нод
#                          кластера из JSON-конфига
#
# Использование:
#   ./update_dns_records.sh [config.json] [dns-server-ip]
# По умолчанию:
#   config.json    — единственный *.json в текущей папке
#   dns-server-ip  — берётся из .defaults.nameserver (первый адрес)
#
# Запускать можно откуда угодно (Proxmox, свой ноутбук) — скрипт
# сам подключается по SSH к base-host-01 и правит зоны там.
# Требует: SSH root@<dns-server-ip> без пароля (ключ уже должен
# быть прописан — см. Network_Migration_Guide.md §6).
# ============================================================

set -e

CONFIG="${1:-}"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

RED='\033[0;31m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[✓]${NC} $1"; }
info() { echo -e "${BLUE}[→]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error(){ echo -e "${RED}[✗]${NC} $1"; exit 1; }

command -v jq &>/dev/null || error "jq не установлен: apt install jq"

if [ -z "$CONFIG" ]; then
    FOUND=($(ls *.json 2>/dev/null))
    if [ "${#FOUND[@]}" -eq 1 ]; then
        CONFIG="${FOUND[0]}"
        warn "Конфиг не указан явно, использую единственный найденный: $CONFIG"
    else
        error "Укажи конфиг явно: $0 <config.json> [dns-server-ip]"
    fi
fi
[ -f "$CONFIG" ] || error "Конфиг не найден: $CONFIG"

DNS_IP="${2:-$(jq -r '.defaults.nameserver' "$CONFIG" | awk '{print $1}')}"
[ -n "$DNS_IP" ] && [ "$DNS_IP" != "null" ] || error "Не удалось определить IP DNS-сервера — укажи вторым аргументом"

VM_COUNT=$(jq '.vms | length' "$CONFIG")

echo "=== Синхронизация DNS-записей на $DNS_IP ==="
echo "  Конфиг: $CONFIG | Нод: $VM_COUNT"
echo ""

# Проверка доступности DNS-сервера по SSH
ssh $SSH_OPTS root@"$DNS_IP" "echo ok" &>/dev/null || error "Нет SSH-доступа к $DNS_IP (root@$DNS_IP)"
log "SSH-доступ к base-host-01 ($DNS_IP) подтверждён"

# Собираем новый блок forward-зоны (A-записи)
FORWARD_BLOCK=""
REVERSE_BLOCK=""
for i in $(seq 0 $((VM_COUNT - 1))); do
    NAME=$(jq -r ".vms[$i].name" "$CONFIG")
    IP=$(jq -r ".vms[$i].ip" "$CONFIG")
    LAST_OCTET=$(echo "$IP" | awk -F. '{print $4}')

    # Выравнивание имени под ширину колонки, как в существующем файле (не критично, но аккуратнее)
    FORWARD_BLOCK="${FORWARD_BLOCK}${NAME}         IN  A    ${IP}\n"
    REVERSE_BLOCK="${REVERSE_BLOCK}${LAST_OCTET}    IN  PTR  ${NAME}.p-stat.org.local.\n"
done

info "Новые записи, которые будут применены:"
echo -e "$FORWARD_BLOCK"
echo -e "$REVERSE_BLOCK"

read -p "Применить эти записи на $DNS_IP? (y/n): " CONFIRM
[ "$CONFIRM" = "y" ] || { warn "Отменено пользователем"; exit 0; }

TIMESTAMP=$(date +%s)

ssh $SSH_OPTS root@"$DNS_IP" bash <<ENDSSH
set -e

FWD_ZONE="/var/lib/named/master/p-stat.org.local.db"
REV_ZONE="/var/lib/named/master/0.168.192.in-addr.arpa.db"

[ -f "\$FWD_ZONE" ] || { echo "Не найден forward-файл зоны: \$FWD_ZONE"; exit 1; }
[ -f "\$REV_ZONE" ] || { echo "Не найден reverse-файл зоны: \$REV_ZONE"; exit 1; }

echo "=== Бэкап текущих зон ==="
cp "\$FWD_ZONE" "\$FWD_ZONE.bak-${TIMESTAMP}"
cp "\$REV_ZONE" "\$REV_ZONE.bak-${TIMESTAMP}"
echo "  Бэкапы: \$FWD_ZONE.bak-${TIMESTAMP}, \$REV_ZONE.bak-${TIMESTAMP}"

echo "=== Обновление forward-зоны ==="
awk -v new_block="$(echo -e "$FORWARD_BLOCK")" '
    /; --- Auto-managed VM entries/ { print; print new_block; skip=1; next }
    /; --- End auto-managed VM entries/ { skip=0 }
    !skip { print }
' "\$FWD_ZONE" > "\$FWD_ZONE.tmp" && mv "\$FWD_ZONE.tmp" "\$FWD_ZONE"

echo "=== Обновление reverse-зоны ==="
awk -v new_block="$(echo -e "$REVERSE_BLOCK")" '
    /; --- Auto-managed VM entries/ { print; print new_block; skip=1; next }
    /; --- End auto-managed VM entries/ { skip=0 }
    !skip { print }
' "\$REV_ZONE" > "\$REV_ZONE.tmp" && mv "\$REV_ZONE.tmp" "\$REV_ZONE"

echo "=== Повышение Serial в обеих зонах ==="
NEW_SERIAL=\$(date +%Y%m%d)01
for ZONE in "\$FWD_ZONE" "\$REV_ZONE"; do
    sed -i -E "s/[0-9]{10} ; Serial.*/\${NEW_SERIAL} ; Serial (auto-updated \$(date '+%Y-%m-%d %H:%M'))/" "\$ZONE"
done

echo "=== Проверка синтаксиса зон ==="
if ! named-checkzone p-stat.org.local "\$FWD_ZONE" || ! named-checkzone 0.168.192.in-addr.arpa "\$REV_ZONE"; then
    echo "✗ Ошибка синтаксиса — откатываю файлы из бэкапа и НЕ перезагружаю BIND"
    cp "\$FWD_ZONE.bak-${TIMESTAMP}" "\$FWD_ZONE"
    cp "\$REV_ZONE.bak-${TIMESTAMP}" "\$REV_ZONE"
    exit 1
fi

echo "=== Перезагрузка BIND ==="
rndc reload || systemctl reload named || systemctl restart named
echo "✓ Зоны обновлены и применены"
ENDSSH

log "DNS-записи синхронизированы на $DNS_IP"

echo ""
info "Проверка резолва с любой ноды кластера (или отсюда, если есть доступ):"
for i in $(seq 0 $((VM_COUNT - 1))); do
    NAME=$(jq -r ".vms[$i].name" "$CONFIG")
    echo "  dig @$DNS_IP $NAME.p-stat.org.local +short"
done
echo "  dig @$DNS_IP -x <IP-ноды> +short   # обратный резолв"
