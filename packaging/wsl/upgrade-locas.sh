#!/usr/bin/env bash
set -Eeuo pipefail
# Parse the whole implementation before replacing this script during an upgrade.
main() {
  local root=/
  umask 022
  archive="${1:?upgrade archive required}"
  [[ "$(id -u)" -eq 0 ]] || { echo "Requires root in LocasOS" >&2; exit 1; }
  [[ -f "$root/etc/systemd/system/locas-core.service" && -x "$root/opt/locas/bin/locas-core" ]] || {
    echo "Not a provisioned LocasOS appliance" >&2; exit 1;
  }
  # This root-owned directory is writable inside the update worker sandbox;
  # /run is read-only under ProtectSystem=strict.
  install -d -m 0700 -o root -g root "$root/var/lib/locas-upgrades"
  exec 9>"$root/var/lib/locas-upgrades/upgrade.lock"
  flock -n 9 || { echo "Another LOCAS upgrade is running" >&2; exit 1; }
  [[ -f "$archive" && -f "${archive}.sha256" ]] || { echo "Upgrade bundle/checksum missing" >&2; exit 1; }
  mapfile -t checksum_lines < "${archive}.sha256"
  [[ "${#checksum_lines[@]}" -eq 1 && "${checksum_lines[0]}" =~ ^([a-fA-F0-9]{64})[[:space:]][\ *](.*)$ ]] || {
    echo "Invalid upgrade checksum format" >&2; exit 1;
  }
  expected_digest="${BASH_REMATCH[1]}"
  expected_file="${BASH_REMATCH[2]}"
  [[ "$expected_file" == "$(basename "$archive")" ]] || { echo "Checksum names another file" >&2; exit 1; }
  actual_digest="$(sha256sum < "$archive")"
  [[ "${actual_digest:0:64}" == "${expected_digest,,}" ]] || {
    echo "Upgrade checksum verification failed" >&2; exit 1;
  }
  work="$(mktemp -d)"
  trap 'rm -rf "$work"' EXIT
  tar -tf "$archive" > "$work/members"
  tar -tvf "$archive" > "$work/types"
  # Only regular files and directories. Reject links before any extraction.
  if grep -qvE '^[-d]' "$work/types"; then
    echo "Links and special files forbidden in upgrade archive" >&2; exit 1
  fi
  declare -A seen=()
  files=()
  while IFS= read -r raw; do
    member="${raw#./}"
    member="${member%/}"
    [[ "$member" =~ ^[a-zA-Z0-9./_-]+$ ]] || { echo "Invalid upgrade member" >&2; exit 1; }
    [[ ! -v "seen[$member]" ]] || { echo "Duplicate upgrade member" >&2; exit 1; }
    seen["$member"]=1
    case "$member" in
      opt|opt/locas|opt/locas/bin|etc|etc/systemd|etc/systemd/system|usr|usr/local|usr/local/sbin) ;;
      opt/locas/bin/locas-core|opt/locas/bin/locas-agent|opt/locas/bin/locas-updater|opt/locas/bin/locas-os-base|\
      etc/systemd/system/locas-core.service|etc/systemd/system/locas-agent.service|\
      etc/systemd/system/locas-app-worker.path|etc/systemd/system/locas-app-worker.service|\
      etc/systemd/system/locas-update-worker.path|etc/systemd/system/locas-update-worker.service|\
      usr/local/sbin/locas-app-worker|usr/local/sbin/locas-install-ollama|\
      usr/local/sbin/locas-install-open-webui|usr/local/sbin/locas-repair-ollama|\
      usr/local/sbin/locas-upgrade) files+=("$member") ;;
      *) echo "Unauthorized upgrade path: $member" >&2; exit 1 ;;
    esac
  done < "$work/members"
  [[ -v 'seen[opt/locas/bin/locas-core]' && -v 'seen[opt/locas/bin/locas-agent]' ]] || {
    echo "Upgrade requires core and agent binaries" >&2; exit 1;
  }
  mkdir "$work/staging"
  tar --no-same-owner --no-same-permissions -C "$work/staging" -xf "$archive"
  for member in "${files[@]}"; do
    [[ -f "$work/staging/$member" && ! -L "$work/staging/$member" ]] || {
      echo "Upgrade payload is not a regular file: $member" >&2; exit 1;
    }
    target="$root/$member"
    while [[ "$target" != / ]]; do
      [[ ! -L "$target" ]] || { echo "Refusing symlink destination: $target" >&2; exit 1; }
      target="$(dirname "$target")"
    done
    [[ ! -e "$root/$member" || -f "$root/$member" ]] || { echo "Invalid destination type" >&2; exit 1; }
  done
  identity="$(cat "$work/staging/opt/locas/bin/locas-os-base" 2>/dev/null || true)"
  source_os="$(. "$root/etc/os-release"; printf '%s:%s' "$ID" "$VERSION_ID")"
  if [[ -z "$identity" || "$identity" != "$source_os" ]]; then
    echo "Blocked cross-distribution LocasOS upgrade: installed=$source_os package=${identity:-unknown}" >&2
    exit 1
  fi
  backup_dir="$root/var/lib/locas-upgrades"
  install -d -m 0700 -o root -g root "$backup_dir"
  backup="$(mktemp -d "$backup_dir/preupgrade-XXXXXXXX")"
  : > "$backup/existing.paths"
  : > "$backup/added.paths"
  : > "$backup/added.dirs"
  for member in "${files[@]}"; do
    if [[ -e "$root/$member" ]]; then
      printf '%s\n' "$member" >> "$backup/existing.paths"
    else
      printf '%s\n' "$member" >> "$backup/added.paths"
    fi
    parent="$(dirname "$root/$member")"
    while [[ ! -d "$parent" ]]; do
      printf '%s\n' "$parent" >> "$backup/added.dirs"
      parent="$(dirname "$parent")"
    done
  done
  tar --no-recursion -C "$root" -cf "$backup/files.tar" -T "$backup/existing.paths"
  chmod 0600 "$backup/"*
  workers=(locas-app-worker.path locas-update-worker.path)
  enabled=(); active=()
  for unit in "${workers[@]}"; do
    enabled+=("$(systemctl is-enabled "$unit" 2>/dev/null || true)")
    active+=("$(systemctl is-active "$unit" 2>/dev/null || true)")
    case "${enabled[-1]}" in
      enabled|disabled|not-found|static|indirect|"") ;;
      *) echo "Unsupported worker enablement state; refusing to change it" >&2; exit 1 ;;
    esac
  done
  restore() {
    result="$1"
    trap - ERR
    set +e
    recovery_failed=false
    echo "Upgrade failed; restoring all replaced LOCAS files" >&2
    for i in "${!workers[@]}"; do
      unit="${workers[$i]}"
      if systemctl is-active --quiet "$unit"; then systemctl stop "$unit" || recovery_failed=true; fi
      if [[ "${enabled[$i]}" != enabled && -f "$root/etc/systemd/system/$unit" ]]; then
        systemctl disable "$unit" || recovery_failed=true
      fi
    done
    while IFS= read -r member; do rm -f -- "$root/$member" || recovery_failed=true; done < "$backup/added.paths"
    tar --no-overwrite-dir -C "$root" -xf "$backup/files.tar" || recovery_failed=true
    # Deepest directories first; rmdir never removes populated/user-data directories.
    sort -ru "$backup/added.dirs" | while IFS= read -r dir; do rmdir -- "$dir" 2>/dev/null || true; done
    systemctl daemon-reload || recovery_failed=true
    for i in "${!workers[@]}"; do
      unit="${workers[$i]}"
      case "${enabled[$i]}" in
        enabled) systemctl enable "$unit" || recovery_failed=true ;;
        *) if [[ -f "$root/etc/systemd/system/$unit" ]]; then systemctl disable "$unit" || recovery_failed=true; fi ;;
      esac
      if [[ "${active[$i]}" == active ]]; then systemctl start "$unit" || recovery_failed=true; fi
    done
    systemctl restart locas-agent.service locas-core.service || recovery_failed=true
    echo "Recovery files retained at $backup" >&2
    if [[ "$recovery_failed" == true ]]; then echo "Automatic recovery incomplete; manual recovery required" >&2; fi
    exit "$result"
  }
  printf '%s %s %s\n' "${workers[0]}" "${enabled[0]}" "${active[0]}" > "$backup/worker.states"
  printf '%s %s %s\n' "${workers[1]}" "${enabled[1]}" "${active[1]}" >> "$backup/worker.states"
  chmod 0600 "$backup/worker.states"
  trap 'restore "$?"' ERR
  # Stop watchers while installing their units/scripts. Never stop a running updater.
  for i in "${!workers[@]}"; do
    if [[ "${active[$i]}" == active ]]; then systemctl stop "${workers[$i]}"; fi
  done
  for member in "${files[@]}"; do
    mkdir -p "$(dirname "$root/$member")"
    case "$member" in etc/systemd/system/*) mode=0644 ;; *) mode=0755 ;; esac
    install -m "$mode" "$work/staging/$member" "$root/$member"
  done
  systemctl daemon-reload
  for unit in "${workers[@]}"; do systemctl enable --now "$unit"; done
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
}
main "$@"
