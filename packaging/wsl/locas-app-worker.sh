#!/usr/bin/env bash
set -Eeuo pipefail
umask 027
dir=/var/lib/locas
request=$dir/app-install.request
status=$dir/app-install.status
[[ -f "$request" && ! -L "$request" ]] || exit 0
IFS= read -r app < "$request" || true
rm -f -- "$request"
write_status() {
  printf '%s\n' "$1" > "$status.tmp"
  chown locas-agent:locas-agent "$status.tmp"
  chmod 0640 "$status.tmp"
  mv -f "$status.tmp" "$status"
}
on_error() { write_status "failed:$app"; }
trap on_error ERR
write_status "installing:$app"
case "$app" in
  ollama)
    if [[ -x /usr/local/bin/ollama ]]; then
      systemctl enable --now locas-ollama.service
    else
      /usr/local/sbin/locas-install-ollama
    fi
    systemctl is-active --quiet locas-ollama.service
    ;;
  open-webui)
    systemctl is-active --quiet locas-ollama.service
    /usr/local/sbin/locas-install-open-webui
    ;;
  *) write_status "rejected"; exit 1 ;;
esac
write_status "installed:$app"
