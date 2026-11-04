#!/usr/bin/env bash
set -Eeuo pipefail
stage=ollama-preflight
work=
# Only fixed stage identifiers cross the worker's diagnostic boundary. Never log
# BASH_COMMAND: installers may handle credentials in command arguments.
trap 'result=$?; if (( result != 0 )); then printf "LOCAS_INSTALL_ERROR=%s\n" "$stage" >&2; fi; [[ -z "$work" ]] || rm -rf -- "$work"; exit "$result"' EXIT
install_stage() { stage=$1; printf 'LOCAS_INSTALL_STAGE=%s\n' "$stage" >&2; }
# Root-only ownership proof. /proc/<pid>/net is namespace-wide, so intersect
# the fixed port's listener inodes with the fixed systemd MainPID's actual FDs.
# Return 0 for owned loopback, 2 for no listener, 3 for conflict/unverifiable.
ollama_listener_state() {
  ollama_main_pid=$(systemctl show --property MainPID --value locas-ollama.service 2>/dev/null) || return 3
  /usr/bin/python3 -I - "$ollama_main_pid" "$ollama_port" <<'PY'
import os
import sys

def state():
    listeners = []
    for table in ('/proc/net/tcp', '/proc/net/tcp6'):
        try:
            with open(table, encoding='ascii') as stream:
                rows = stream.readlines()[1:]
        except FileNotFoundError:
            if table.endswith('tcp6'):
                continue
            raise
        for row in rows:
            fields = row.split()
            if len(fields) < 10:
                raise ValueError('invalid socket table')
            address, port = fields[1].split(':')
            width = 32 if table.endswith('tcp6') else 8
            if (len(address) != width or len(port) != 4 or len(fields[3]) != 2
                    or any(c not in '0123456789ABCDEF' for c in address + port + fields[3])
                    or not fields[9].isascii() or not fields[9].isdecimal()):
                raise ValueError('invalid socket identity')
            if fields[3] != '0A' or port != f'{int(sys.argv[2]):04X}':
                continue
            # Wildcard/v4-mapped listeners can conflict with 127.0.0.1 too.
            if address in ('0100007F', '00000000', '0' * 32,
                           '0000000000000000FFFF00000100007F'):
                listeners.append((address, fields[9]))
    if not listeners:
        return 2
    pid = sys.argv[1]
    if not pid.isascii() or not pid.isdecimal() or int(pid) <= 0:
        return 3
    owned = set()
    for entry in os.scandir('/proc/' + pid + '/fd'):
        try:
            target = os.readlink(entry.path)
        except FileNotFoundError:
            continue  # An unrelated FD may close while enumerating.
        if target.startswith('socket:[') and target.endswith(']'):
            owned.add(target[8:-1])
    return 0 if all(address == '0100007F' and inode in owned
                    for address, inode in listeners) else 3

try:
    result = state()
except (OSError, ValueError, IndexError):
    result = 3
sys.exit(result)
PY
}
install_stage ollama-preflight
ollama_host=${OLLAMA_HOST-127.0.0.1:11434}
if [[ ! "$ollama_host" =~ ^127[.]0[.]0[.]1:([0-9]{1,5})$ ]]; then
  echo "Invalid Ollama listener configuration; require a loopback IPv4 address and an allowed port." >&2
  exit 1
fi
ollama_port=$((10#${BASH_REMATCH[1]}))
if (( ollama_port < 1024 || ollama_port > 65535 || ollama_port == 59420 || ollama_port == 59421 )); then
  echo "Invalid Ollama listener port; preserve the appliance dashboard and workspace ports." >&2
  exit 1
fi
ollama_host=127.0.0.1:$ollama_port
# Only the dedicated LocasOS WSL distro; installed by explicit user opt-in.
if [[ $(id -u) -ne 0 ]]; then
  echo "Must run as root in LocasOS" >&2
  exit 1
fi
if [[ ! -f /etc/wsl.conf ]] || ! grep -q 'systemd=true' /etc/wsl.conf || ! systemctl list-unit-files --no-legend locas-core.service | grep -q locas-core; then
  echo "Refusing non-LocasOS distro" >&2
  exit 1
fi
# Check before downloads, extraction or service changes. Never stop or replace a
# foreign listener, and never let its healthy API stand in for our failing unit.
if ollama_listener_state; then
  listener_state=0
else
  listener_state=$?
fi
if (( listener_state != 2 )) && { (( listener_state != 0 )) || ! systemctl is-active --quiet locas-ollama.service; }; then
  install_stage ollama-port-conflict
  echo "Cannot verify exclusive LocasOS ownership of the configured Ollama listener. Preserve other services and inspect the listener owner." >&2
  exit 1
fi
if [[ -e /usr/local/bin/ollama ]]; then
  # A prior attempt may have installed the runtime before service startup failed.
  # Retry startup without overwriting binaries, models or configuration.
  if [[ ! -x /usr/local/bin/ollama || ! -x /usr/local/lib/ollama/llama-server ]]; then
    echo "Existing Ollama runtime is incomplete; use the explicit runtime repair." >&2
    exit 1
  fi
  echo "Existing Ollama runtime preserved; checking service readiness."
else
  work="$(mktemp -d)"
  url="https://ollama.com/download/ollama-linux-amd64.tar.zst"
  install_stage ollama-download
  echo "Downloading official Ollama Linux archive (runtime version supplied by ollama.com)"
  curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --output "$work/ollama.tar.zst" "$url"
  # GNU tar lists archive members as either bin/ollama or ./bin/ollama.
  # Avoid piping tar to grep with pipefail: an early grep exit would signal SIGPIPE.
  install_stage ollama-archive
  tar --zstd -tf "$work/ollama.tar.zst" > "$work/contents.txt"
  if ! grep -Eq '^(\./)?bin/ollama$' "$work/contents.txt"; then
    echo "Unexpected archive layout; required Ollama binary is missing." >&2
    exit 1
  fi
  if ! grep -Eq '^(\./)?lib/ollama/llama-server$' "$work/contents.txt"; then
    echo "Official Ollama archive missing required llama-server; aborting installation" >&2
    exit 1
  fi
  install_stage ollama-extract
  tar --zstd -xf "$work/ollama.tar.zst" -C /usr/local
  if [[ ! -x /usr/local/bin/ollama || ! -x /usr/local/lib/ollama/llama-server ]]; then
    echo "Ollama binary or required runner missing after extraction" >&2
    exit 1
  fi
fi
install_stage ollama-service
systemctl daemon-reload
systemctl enable --now locas-ollama.service
systemctl is-active --quiet locas-ollama.service
install_stage ollama-readiness
ready=false
for ((attempt=1; attempt<=30; attempt++)); do
  if systemctl is-active --quiet locas-ollama.service &&
    ollama_listener_state &&
    probe_pid=$ollama_main_pid &&
    http_status=$(curl --silent --noproxy '*' --proto '=http' --connect-timeout 1 --max-time 2 \
      --output /dev/null --write-out '%{http_code}' "http://$ollama_host/api/tags") &&
    [[ "$http_status" == 200 ]] &&
    ollama_listener_state && [[ "$ollama_main_pid" == "$probe_pid" ]] &&
    systemctl is-active --quiet locas-ollama.service; then
    ready=true
    break
  fi
  (( attempt == 30 )) || sleep 1
done
if [[ "$ready" != true ]]; then
  echo "Ollama did not become ready at its local API; inspect locas-ollama.service." >&2
  exit 1
fi
echo "Installed isolated Ollama runtime. Live archive download is not checksum-pinned."
