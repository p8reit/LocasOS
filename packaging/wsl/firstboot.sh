#!/usr/bin/env bash
set -Eeuo pipefail
umask 027
install -d -m 0750 -o locas-agent -g locas-agent /var/lib/locas
install -d -m 0750 -o locas-agent -g locas-agent /run/locas
# The agent only accepts a known dedicated service identity.
core_uid="$(id -u locas-core)"
if [[ ! "$core_uid" =~ ^[0-9]+$ ]]; then
  echo "LOCAS: invalid service UID" >&2
  exit 1
fi
install -d -m 0755 /etc/locas
printf 'LOCAS_CORE_UID=%s\n' "$core_uid" > /etc/locas/agent.env.tmp
chmod 0640 /etc/locas/agent.env.tmp
chown root:locas-agent /etc/locas/agent.env.tmp
mv -f /etc/locas/agent.env.tmp /etc/locas/agent.env
# No Docker socket is connected and no password/token is generated at this stage.
