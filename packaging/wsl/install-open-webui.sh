#!/usr/bin/env bash
set -Eeuo pipefail
# Dedicated LocasOS distribution only. Requires explicit opt-in; never accesses host Docker Desktop.
[[ "$(id -u)" == 0 ]] || { echo "Run as root" >&2; exit 1; }
[[ -f /etc/systemd/system/locas-core.service ]] || { echo "Not LocasOS" >&2; exit 1; }
systemctl is-active --quiet locas-ollama.service || { echo "Start locas-ollama first" >&2; exit 1; }
if [[ -S /var/run/docker.sock ]] && ! systemctl is-active --quiet docker.service; then
  echo "An unmanaged Docker socket is present. Refusing to use it." >&2
  exit 1
fi
if ! command -v docker >/dev/null; then
  echo "Installing isolated Docker Engine in LocasOS (not Docker Desktop)"
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends docker.io
fi
systemctl enable --now docker.service
systemctl is-active --quiet docker.service
mkdir -p /var/lib/locas-open-webui/data
chmod 0700 /var/lib/locas-open-webui
keyfile=/var/lib/locas-open-webui/secret
if [[ ! -f "$keyfile" ]]; then
  umask 077
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$keyfile"
fi
if docker container inspect locas-open-webui >/dev/null 2>&1; then
  echo "LOCAS Open WebUI container already exists; not replacing its data or configuration."
  docker start locas-open-webui || true
else
  image=ghcr.io/open-webui/open-webui:main-slim
  docker pull "$image"
  docker run -d --name locas-open-webui --restart unless-stopped \
    --network host --cap-drop ALL --security-opt no-new-privileges:true \
    --pids-limit 256 --memory 3g \
    --mount type=bind,source=/var/lib/locas-open-webui/data,target=/app/backend/data \
    -e HOST=127.0.0.1 -e PORT=59421 \
    -e OLLAMA_BASE_URL=http://127.0.0.1:11434 \
    -e ENABLE_OPENAI_API=False -e ENABLE_COMMUNITY_SHARING=False \
    -e WEBUI_NAME='LOCAS OS Workspace' \
    -e WEBUI_SECRET_KEY="$(cat "$keyfile")" \
    "$image"
fi
echo "Open WebUI container started on http://127.0.0.1:59421/. First Open WebUI account is its admin."
echo "No external inference provider has been configured; agent tools are not installed or enabled."
