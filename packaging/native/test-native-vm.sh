#!/usr/bin/env bash
# Guest-local acceptance check; run after first install AND after reboot.
set -Eeuo pipefail
if grep -qi microsoft /proc/sys/kernel/osrelease; then
  echo 'This test is for native VMs, not WSL' >&2; exit 1
fi
if ! command -v systemctl >/dev/null; then echo 'systemd required' >&2; exit 1; fi
for service in locas-firstboot locas-agent locas-core; do
  if ! systemctl is-active --quiet "${service}.service"; then
    systemctl --no-pager status "${service}.service" || true
    journalctl -b -u "${service}.service" --no-pager -n 60 || true
    echo "FAIL: $service is not running" >&2
    exit 1
  fi
done
health="$(curl -fsS --max-time 5 --noproxy '*' http://127.0.0.1:59420/healthz)"
python3 -c 'import json,sys; h=json.loads(sys.argv[1]); assert h.get("component")=="locas-core" and h.get("status")=="ok", h' "$health"
if ! systemctl is-enabled --quiet locas-agent.service locas-core.service; then
  echo 'FAIL: services not enabled for reboot' >&2; exit 1
fi
if [[ "$(ss -ltn '( sport = :59420 )' | sed -n '2p')" != *127.0.0.1:59420* ]]; then
  echo 'FAIL: control plane not bound to loopback' >&2; exit 1
fi
echo 'PASS: native VM service activation, loopback HTTP, reboot persistence configuration'
