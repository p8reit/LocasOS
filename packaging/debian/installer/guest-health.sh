#!/bin/sh
# Live-guest readiness beacon; available only in disposable native development images.
set -eu
for attempt in $(seq 1 120); do
  if systemctl is-active --quiet locas-agent.service locas-core.service &&
     curl --noproxy '*' -fsS --max-time 2 http://127.0.0.1:59420/healthz | grep -q '"status":"ok"'; then
    printf '\nLOCASOS_GUEST_HEALTH_OK\n' > /dev/ttyS0
    exit 0
  fi
  sleep 1
done
printf '\nLOCASOS_GUEST_HEALTH_FAILED\n' > /dev/ttyS0
exit 1
