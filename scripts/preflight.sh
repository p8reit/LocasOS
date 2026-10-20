#!/usr/bin/env bash
set -u
# Read-only host diagnostics: no installations, restarts or configuration changes.
printf 'LOCAS OS host preflight (read-only)\n'
issues=0
check() {
  local title="$1"; shift
  if "$@" >/dev/null 2>&1; then printf '[OK] %s\n' "$title"
  else printf '[MISSING] %s\n' "$title"; issues=$((issues+1)); fi
}
check 'Linux kernel' test "$(uname -s)" = Linux
check 'systemd installed' command -v systemctl
check 'Docker CLI' command -v docker
check 'Docker Compose v2' bash -c 'docker compose version'
check 'NVIDIA SMI' command -v nvidia-smi
check 'NVIDIA GPU accessible' bash -c 'nvidia-smi -L'
check 'NVIDIA Container Toolkit' command -v nvidia-ctk
printf 'Missing prerequisites: %d\n' "$issues"
if ((issues)); then exit 1; fi
