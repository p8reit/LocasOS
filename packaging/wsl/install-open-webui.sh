#!/usr/bin/env bash
set -Eeuo pipefail
stage=webui-preflight
work=
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
install_stage webui-preflight
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
# Dedicated LocasOS distribution only. Requires explicit opt-in; never accesses host Docker Desktop.
[[ "$(id -u)" == 0 ]] || { echo "Run as root" >&2; exit 1; }
[[ -f /etc/systemd/system/locas-core.service ]] || { echo "Not LocasOS" >&2; exit 1; }
systemctl is-active --quiet locas-ollama.service || { echo "Start locas-ollama first" >&2; exit 1; }
if ! ollama_listener_state; then
  echo "Ollama's listener is not owned by its LocasOS service; inspect the listener owner." >&2
  exit 1
fi
probe_pid=$ollama_main_pid
if ! http_status=$(curl --silent --noproxy '*' --proto '=http' --connect-timeout 1 --max-time 2 \
  --output /dev/null --write-out '%{http_code}' "http://$ollama_host/api/tags") || [[ "$http_status" != 200 ]] ||
  ! ollama_listener_state || [[ "$ollama_main_pid" != "$probe_pid" ]] ||
  ! systemctl is-active --quiet locas-ollama.service; then
  echo "Ollama's local API is not ready; verify locas-ollama before installing Open WebUI." >&2
  exit 1
fi
if [[ -S /var/run/docker.sock ]] && ! systemctl is-active --quiet docker.service; then
  echo "An unmanaged Docker socket is present. Refusing to use it." >&2
  exit 1
fi
if ! command -v docker >/dev/null || ! command -v dockerd >/dev/null; then
  install_stage webui-docker-install
  echo "Installing isolated Docker Engine in LocasOS (not Docker Desktop)"
  apt-get update
  # Debian 13 ships /usr/bin/docker in a separate recommended-only package.
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends docker.io docker-cli
  command -v docker >/dev/null
fi
# Use only this distribution's systemd-managed daemon, regardless of inherited
# Docker client context, host or TLS settings.
unset DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH
docker() { command docker --host unix:///var/run/docker.sock "$@"; }
install_stage webui-docker-service
systemctl enable --now docker.service
systemctl is-active --quiet docker.service
install_stage webui-storage
mkdir -p /var/lib/locas-open-webui/data
chmod 0700 /var/lib/locas-open-webui
keyfile=/var/lib/locas-open-webui/secret
if [[ ! -f "$keyfile" ]]; then
  umask 077
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$keyfile"
fi
if docker container inspect locas-open-webui >/dev/null 2>&1; then
  echo "LOCAS Open WebUI container already exists; not replacing its data or configuration."
  install_stage webui-runtime-config
  # Select only this non-secret key. Never inspect/log the complete environment,
  # and never silently recreate an existing container to change its endpoint.
  if ! configured_runtime=$(docker container inspect --format \
    '{{range .Config.Env}}{{if eq (index (split . "=") 0) "OLLAMA_BASE_URL"}}{{println .}}{{end}}{{end}}' \
    locas-open-webui 2>/dev/null) || [[ "$configured_runtime" != "OLLAMA_BASE_URL=http://$ollama_host" ]]; then
    echo "Existing Open WebUI runtime endpoint differs or cannot be verified. Explicitly reconcile its configuration while preserving data." >&2
    exit 1
  fi
  install_stage webui-container-start
  docker start locas-open-webui
else
  image=ghcr.io/open-webui/open-webui:main-slim
  install_stage webui-pull
  docker pull "$image"
  install_stage webui-storage
  # Keep the secret out of docker's process arguments and the worker journal.
  # Existing files are never regenerated, including on a failed retry.
  if [[ ! $(cat "$keyfile") =~ ^[[:xdigit:]]{64}$ ]]; then
    echo "Existing Open WebUI secret has an unexpected format; preserve it for inspection." >&2
    exit 1
  fi
  umask 077
  work="$(mktemp -d)"
  printf 'WEBUI_SECRET_KEY=%s\n' "$(cat "$keyfile")" > "$work/container.env"
  install_stage webui-container-start
  docker run -d --name locas-open-webui --restart unless-stopped \
    --network host --cap-drop ALL --security-opt no-new-privileges:true \
    --pids-limit 256 --memory 3g \
    --mount type=bind,source=/var/lib/locas-open-webui/data,target=/app/backend/data \
    -e HOST=127.0.0.1 -e PORT=59421 \
    -e "OLLAMA_BASE_URL=http://$ollama_host" \
    -e ENABLE_OPENAI_API=False -e ENABLE_COMMUNITY_SHARING=False \
    -e WEBUI_NAME='LOCAS OS Workspace' \
    --env-file "$work/container.env" \
    "$image"
fi
install_stage webui-readiness
ready=false
for ((attempt=1; attempt<=90; attempt++)); do
  # Read one fixed non-secret field; never print raw inspect output/environment.
  if ! running=$(docker container inspect --format '{{.State.Running}}' locas-open-webui 2>/dev/null) || [[ "$running" != true ]]; then
    echo "Open WebUI container is not running; inspect its local service logs." >&2
    exit 1
  fi
  if http_status=$(curl --silent --noproxy '*' --proto '=http' --connect-timeout 1 --max-time 3 \
    --output /dev/null --write-out '%{http_code}' http://127.0.0.1:59421/health) &&
    [[ "$http_status" == 200 ]]; then
    ready=true
    break
  fi
  (( attempt == 90 )) || sleep 2
done
if [[ "$ready" != true ]]; then
  echo "Open WebUI did not become ready at its local health endpoint." >&2
  exit 1
fi
echo "Open WebUI container started on http://127.0.0.1:59421/. First Open WebUI account is its admin."
echo "No external inference provider has been configured; agent tools are not installed or enabled."
