#!/usr/bin/env bash
set -Eeuo pipefail
[[ "$(id -u)" -eq 0 && -f /etc/systemd/system/locas-ollama.service ]] || { echo "Run as root in LocasOS" >&2; exit 1; }
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
echo "Downloading full official Ollama Linux runtime (models will remain untouched)."
curl -fL --retry 3 --proto '=https' --tlsv1.2 -o "$work/ollama.tar.zst" https://ollama.com/download/ollama-linux-amd64.tar.zst
tar --zstd -tf "$work/ollama.tar.zst" > "$work/manifest"
grep -Eq '^([.]/)?bin/ollama$' "$work/manifest" || { echo "Missing ollama binary in archive" >&2; exit 1; }
grep -Eq '^([.]/)?lib/ollama/llama-server$' "$work/manifest" || { echo "Official archive lacks required llama-server; refusing replacement" >&2; exit 1; }
mkdir "$work/staging"
tar --zstd -xf "$work/ollama.tar.zst" -C "$work/staging"
[[ -x "$work/staging/bin/ollama" && -x "$work/staging/lib/ollama/llama-server" ]] || { echo "Archive runtime incomplete" >&2; exit 1; }
systemctl stop locas-ollama.service
# Keep rollback of installed runtime; DO NOT touch /var/lib/locas-ollama/models.
if [[ -d /usr/local/lib/ollama ]]; then mv /usr/local/lib/ollama "$work/old-lib"; fi
if [[ -f /usr/local/bin/ollama ]]; then cp -a /usr/local/bin/ollama "$work/old-bin"; fi
rollback() {
  echo "Restoring previous Ollama runtime after failure" >&2
  rm -rf /usr/local/lib/ollama
  [[ ! -d "$work/old-lib" ]] || mv "$work/old-lib" /usr/local/lib/ollama
  [[ ! -f "$work/old-bin" ]] || cp -a "$work/old-bin" /usr/local/bin/ollama
  systemctl start locas-ollama.service || true
}
trap rollback ERR
install -d -m 0755 /usr/local/lib/ollama /usr/local/bin
cp -a "$work/staging/lib/ollama/." /usr/local/lib/ollama/
install -m 0755 "$work/staging/bin/ollama" /usr/local/bin/ollama
systemctl start locas-ollama.service
systemctl is-active --quiet locas-ollama.service
[[ -x /usr/local/lib/ollama/llama-server ]]
trap - ERR
echo "Full Ollama runtime repaired. Existing model library preserved."
