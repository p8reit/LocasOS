#!/usr/bin/env bash
set -Eeuo pipefail
if [[ "$(id -u)" -ne 0 ]] || [[ ! -f /etc/systemd/system/locas-ollama.service ]]; then
  echo "Requires root inside an installed LocasOS appliance" >&2
  exit 1
fi
unit=/etc/systemd/system/locas-ollama.service
if ! grep -q '^Environment=HOME=/var/lib/locas-ollama$' "$unit"; then
  sed -i '/^Environment=OLLAMA_NOHISTORY=1$/a Environment=HOME=/var/lib/locas-ollama' "$unit"
fi
install -d -m 0700 -o locas-ollama -g locas-ollama /var/lib/locas-ollama
install -d -m 0700 -o locas-ollama -g locas-ollama /var/lib/locas-ollama/.ollama
install -d -m 0700 -o locas-ollama -g locas-ollama /var/lib/locas-ollama/models
systemctl daemon-reload
systemctl restart locas-ollama.service
systemctl is-active --quiet locas-ollama.service
systemctl show locas-ollama.service --property=Environment --no-pager
echo "Ollama home/state permission repair complete; no model re-download performed."
