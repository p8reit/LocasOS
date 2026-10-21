#!/usr/bin/env bash
set -Eeuo pipefail
# Explicitly invoked by Windows opt-in script. Never install in shared Ubuntu/Docker.
if [[ $(id -u) -ne 0 ]]; then echo "Must run as root in LocasOS" >&2; exit 1; fi
if [[ ! -f /etc/wsl.conf ]] || ! grep -q 'systemd=true' /etc/wsl.conf || ! systemctl list-unit-files --no-legend locas-core.service | grep -q locas-core; then
  echo "Refusing non-LocasOS distro" >&2; exit 1
fi
if [[ -e /usr/local/bin/ollama ]]; then
  echo "Ollama already installed; refusing to overwrite an existing runtime" >&2
  exit 1
fi
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
url="https://ollama.com/download/ollama-linux-amd64.tar.zst"
echo "Downloading official Ollama Linux archive (runtime version supplied by ollama.com)"
curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --output "$work/ollama.tar.zst" "$url"
# Download and extraction deliberately happen only after the explicit opt-in.
tar --zstd -tf "$work/ollama.tar.zst" | grep -qx 'bin/ollama' || {
  echo "Unexpected archive layout" >&2; exit 1;
}
tar --zstd -xf "$work/ollama.tar.zst" -C /usr/local
test -x /usr/local/bin/ollama || { echo "Ollama binary missing after extraction" >&2; exit 1; }
systemctl daemon-reload
systemctl enable --now locas-ollama.service
systemctl is-active --quiet locas-ollama.service
echo "Installed isolated Ollama runtime. Note: live downloaded archive is not pinned to a release checksum."
