#!/usr/bin/env bash
set -Eeuo pipefail
# Run as: bash -s -- /mnt/<drive>/.../locasos-upgrade-amd64.tar
archive="${1:?upgrade archive required}"
[[ "$(id -u)" -eq 0 ]] || { echo "Requires root in LocasOS" >&2; exit 1; }
[[ -f /etc/systemd/system/locas-core.service && -x /opt/locas/bin/locas-core ]] || {
  echo "Not a provisioned LocasOS appliance" >&2; exit 1;
}
[[ -f "$archive" ]] || { echo "Upgrade bundle not found: $archive" >&2; exit 1; }
checksum="${archive}.sha256"
[[ -f "$checksum" ]] || { echo "Missing archive checksum" >&2; exit 1; }
(cd "$(dirname "$archive")" && sha256sum -c "$(basename "$checksum")") || {
  echo "Upgrade checksum verification failed" >&2; exit 1;
}
members="$(mktemp)"
trap 'rm -f "$members"' EXIT
tar -tf "$archive" > "$members"
if grep -Eq '^([.]/?)
grep -qx './opt/locas/bin/locas-core' "$members" || {
  echo "Unexpected bundle: missing LOCAS core" >&2; exit 1;
}
# Upgrade bundles are trusted release inputs. Refuse traversal paths and links.
if grep -Eq '(^/|(^|/)\.\.(/|$))' "$members"; then
  echo "Unsafe paths in archive" >&2; exit 1
fi
backup_dir=/var/lib/locas-upgrades
install -d -m 0700 -o root -g root "$backup_dir"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup="$backup_dir/preupgrade-$stamp.tar"
tar -C / -cf "$backup" opt/locas/bin etc/systemd/system/locas-core.service etc/systemd/system/locas-agent.service
chmod 0600 "$backup"
restore() {
  echo "Upgrade failed, restoring prior executables and core service units" >&2
  tar -C / -xf "$backup" || true
  systemctl daemon-reload || true
  systemctl restart locas-agent.service locas-core.service || true
}
trap restore ERR
# Retain /var/lib/locas-core, /var/lib/locas-ollama, /var/lib/locas-open-webui,
# /var/lib/docker, and existing administrator and user data.
tar --no-overwrite-dir -C / -xf "$archive"
chmod 0755 /opt/locas/bin/locas-core /opt/locas/bin/locas-agent
systemctl daemon-reload
systemctl enable --now locas-app-worker.path
systemctl restart locas-agent.service
systemctl restart locas-core.service
ready=false
for attempt in $(seq 1 30); do
  if systemctl is-active --quiet locas-agent.service &&
     systemctl is-active --quiet locas-core.service &&
     curl --fail --silent --max-time 2 http://127.0.0.1:59420/healthz >/dev/null; then
    ready=true
    break
  fi
  sleep 2
done
if [[ "$ready" != true ]]; then
  echo "LOCAS services did not pass readiness checks after 60 seconds" >&2
  systemctl --no-pager -l status locas-agent.service locas-core.service >&2 || true
  journalctl -b -u locas-core.service -u locas-agent.service -n 50 --no-pager >&2 || true
  false
fi
trap - ERR
printf 'In-place upgrade succeeded. Backup: %s\n' "$backup"
 "$members"; then
  echo "Unsafe archive: top-level dot entry could modify filesystem root permissions" >&2; exit 1
fi
grep -qx './opt/locas/bin/locas-core' "$members" || {
  echo "Unexpected bundle: missing LOCAS core" >&2; exit 1;
}
# Upgrade bundles are trusted release inputs. Refuse traversal paths and links.
if grep -Eq '(^/|(^|/)\.\.(/|$))' "$members"; then
  echo "Unsafe paths in archive" >&2; exit 1
fi
backup_dir=/var/lib/locas-upgrades
install -d -m 0700 -o root -g root "$backup_dir"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup="$backup_dir/preupgrade-$stamp.tar"
tar -C / -cf "$backup" opt/locas/bin etc/systemd/system/locas-core.service etc/systemd/system/locas-agent.service
chmod 0600 "$backup"
restore() {
  echo "Upgrade failed, restoring prior executables and core service units" >&2
  tar -C / -xf "$backup" || true
  systemctl daemon-reload || true
  systemctl restart locas-agent.service locas-core.service || true
}
trap restore ERR
# Retain /var/lib/locas-core, /var/lib/locas-ollama, /var/lib/locas-open-webui,
# /var/lib/docker, and existing administrator and user data.
tar -C / -xf "$archive"
chmod 0755 /opt/locas/bin/locas-core /opt/locas/bin/locas-agent
systemctl daemon-reload
systemctl enable --now locas-app-worker.path
systemctl restart locas-agent.service
systemctl restart locas-core.service
ready=false
for attempt in $(seq 1 30); do
  if systemctl is-active --quiet locas-agent.service &&
     systemctl is-active --quiet locas-core.service &&
     curl --fail --silent --max-time 2 http://127.0.0.1:59420/healthz >/dev/null; then
    ready=true
    break
  fi
  sleep 2
done
if [[ "$ready" != true ]]; then
  echo "LOCAS services did not pass readiness checks after 60 seconds" >&2
  systemctl --no-pager -l status locas-agent.service locas-core.service >&2 || true
  journalctl -b -u locas-core.service -u locas-agent.service -n 50 --no-pager >&2 || true
  false
fi
trap - ERR
printf 'In-place upgrade succeeded. Backup: %s\n' "$backup"
