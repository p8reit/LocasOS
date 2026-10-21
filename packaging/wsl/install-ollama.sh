#!/usr/bin/env bash
set -Eeuo pipefail
# Only the dedicated LocasOS WSL distro; installed by explicit user opt-in.
if [[ $(id -u) -ne 0 ]]; then
  echo "Must run as root in LocasOS" >&2
  exit 1
fi
if [[ ! -f /etc/wsl.conf ]] || ! grep -q 'systemd=true' /etc/wsl.conf || ! systemctl list-unit-files --no-legend locas-core.service | grep -q locas-core; then
  echo "Refusing non-LocasOS distro" >&2
  exit 1
fi
if [[ -e /usr/local/bin/ollama ]]; then
  echo "Ollama already installed; refusing to overwrite" >&2
  exit 1
fi
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
url="https://ollama.com/download/ollama-linux-amd64.tar.zst"
echo "Downloading official Ollama Linux archive (runtime version supplied by ollama.com)"
curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --output "$work/ollama.tar.zst" "$url"
# GNU tar lists archive members as either bin/ollama or ./bin/ollama.
# Avoid piping tar to grep with pipefail: an early grep exit would signal SIGPIPE.
tar --zstd -tf "$work/ollama.tar.zst" > "$work/contents.txt"
if ! grep -Eq '^(\\./)?bin/ollama$' "$work/contents.txt"; then
  echo "Unexpected archive layout. First entries:" >&2
  head -n 15 "$work/contents.txt" >&2
  exit 1
fi
tar --zstd -xf "$work/ollama.tar.zst" -C /usr/local
if [[ ! -x /usr/local/bin/ollama ]]; then
  echo "Ollama binary missing after extraction" >&2
  exit 1
fi
systemctl daemon-reload
systemctl enable --now locas-ollama.service
systemctl is-active --quiet locas-ollama.service
echo "Installed isolated Ollama runtime. Live archive download is not checksum-pinned."
