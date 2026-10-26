#!/usr/bin/env bash
set -Eeuo pipefail
[[ "$(id -u)" == 0 ]] || { echo 'Root required' >&2; exit 1; }
[[ -f /etc/wsl.conf && -f /etc/systemd/system/locas-core.service ]] || { echo 'Not LocasOS WSL' >&2; exit 1; }
. /etc/os-release
printf 'LOCASOS_INVENTORY_VERSION=1\nOS_ID=%s\nOS_VERSION=%s\n' "$ID" "$VERSION_ID"
for path in /var/lib/locas /var/lib/locas-core /var/lib/locas-ollama /var/lib/locas-updates /var/lib/locas-upgrades /etc/locas /var/lib/docker /var/lib/containers /home; do
 if [[ -e "$path" ]]; then
  bytes="$(du -sB1 -- "$path" 2>/dev/null | awk '{print $1}')"
  printf 'PRESENT\t%s\t%s\n' "$path" "$bytes"
 else
  printf 'ABSENT\t%s\n' "$path"
 fi
done
for service in locas-firstboot locas-agent locas-core locas-ollama; do
 printf 'SERVICE\t%s\t%s\n' "$service" "$(systemctl is-active "$service.service" 2>/dev/null || true)"
done
printf 'NOTE=Read-only; contents and secret values not collected\n'
