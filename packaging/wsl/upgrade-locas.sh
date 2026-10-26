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
if grep -Eq '^([.]/?)$' "$members"; then
  echo "Unsafe archive: filesystem root entry" >&2
  exit 1
fi
grep -Eq '^(\\./)?opt/locas/bin/locas-core$' "$members" || {
  echo "Unexpected upgrade bundle: missing LOCAS core" >&2; exit 1;
}
if grep -Eq '(^/|(^|/)\\.\\.(/|$))' "$members"; then
  echo "Unsafe archive traversal detected" >&2; exit 1
fi
# Do not perform cross-distribution application upgrades. In particular, NEVER
# apply Debian-compiled artifacts over the existing Ubuntu 24.04 stable distro.
identity="$(tar -xOf "$archive" opt/locas/bin/locas-os-base 2>/dev/null || true)"
source_os="$(. /etc/os-release; printf '%s:%s' "$ID" "$VERSION_ID")"
if [[ "$identity" != "$source_os" ]]; then
  echo "Blocked cross-distribution LocasOS upgrade: installed=$source_os package=${identity:-unknown}. Use a fresh signed distro install and separately tested data migration." >&2
  exit 1
fi
# Snapshot every pre-existing file the package can replace, not only Core/Agent.
# Record package additions separately so a failed installation can remove them.
backup_dir=/var/lib/locas-upgrades
install -d -m 0700 -o root -g root "$backup_dir"
stamp="$(date -u +%Y%m%dT%H%M%SZ)-$$"
backup="$backup_dir/preupgrade-$stamp.tar"
existing="$(mktemp)"
added="$(mktemp)"
cleanup() { rm -f "$members" "$existing" "$added"; }
trap cleanup EXIT

while IFS= read -r member; do
  member="${member#./}"
  member="${member%/}"
  [[ -n "$member" ]] || continue
  # The upgrade package is confined to product-owned files and service units.
  case "$member" in
    opt|opt/locas|opt/locas/bin|etc|etc/systemd|etc/systemd/system|usr|usr/local|usr/local/sbin) continue ;;
    opt/locas/bin/*|etc/systemd/system/locas-*.service|etc/systemd/system/locas-*.path|usr/local/sbin/locas-*) ;;
    *) echo "Unauthorized archive member: $member" >&2; exit 1 ;;
  esac
  if [[ -L "/$member" ]]; then
    echo "Refusing to replace symbolic link: $member" >&2; exit 1
  elif [[ -f "/$member" ]]; then
    printf '%s\n' "$member" >> "$existing"
  elif [[ ! -e "/$member" ]]; then
    printf '%s\n' "$member" >> "$added"
  else
    echo "Unexpected existing archive destination: $member" >&2; exit 1
  fi
done < "$members"
tar -C / -cf "$backup" -T "$existing"
chmod 0600 "$backup"
restore() {
  trap - ERR
  echo "Upgrade failed; restoring previous LOCAS files and removing additions" >&2
  local failed=0
  while IFS= read -r member; do
    rm -f -- "/$member" || failed=1
  done < "$added"
  tar -C / -xf "$backup" || failed=1
  systemctl daemon-reload || failed=1
  systemctl restart locas-agent.service locas-core.service || failed=1
  if [[ "$failed" -ne 0 ]]; then
    echo "CRITICAL: Rollback incomplete. Preserve backup: $backup" >&2
  fi
}
trap restore ERR
tar --no-overwrite-dir -C / -xf "$archive"
chmod 0755 /opt/locas/bin/locas-core /opt/locas/bin/locas-agent
systemctl daemon-reload
systemctl enable --now locas-app-worker.path
systemctl enable --now locas-update-worker.path
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
  echo "LOCAS Core/Agent did not become healthy" >&2
  systemctl --no-pager -l status locas-agent.service locas-core.service >&2 || true
  journalctl -b -u locas-core.service -u locas-agent.service -n 50 --no-pager >&2 || true
  false
fi
trap - ERR
printf 'In-place upgrade succeeded. Backup: %s\n' "$backup"
