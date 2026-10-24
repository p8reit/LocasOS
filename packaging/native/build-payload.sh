#!/usr/bin/env bash
# Build a portable application payload, NOT a bootable ISO or installer.
set -Eeuo pipefail
umask 027
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"
mkdir -p build dist/native
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o build/locas-core ./cmd/locas-core
go build -trimpath -ldflags="-s -w" -o build/locas-agent ./cmd/locas-agent
# The updater's verification key MUST be injected from trusted release config.
if [[ -z "${LOCAS_UPDATE_PUBLIC_KEY_B64:-}" ]]; then
  echo 'LOCAS_UPDATE_PUBLIC_KEY_B64 missing; refuse to build an updater without a pinned key' >&2
  exit 1
fi
go build -trimpath -ldflags="-s -w -X main.trustedPublicKeyBase64=${LOCAS_UPDATE_PUBLIC_KEY_B64}" -o build/locas-updater ./cmd/locas-updater
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/opt/locas/bin" "$stage/usr/local/sbin" "$stage/etc/systemd/system"
install -m 0755 build/locas-core build/locas-agent build/locas-updater "$stage/opt/locas/bin/"
for f in firstboot.sh locas-app-worker.sh install-ollama.sh install-open-webui.sh repair-ollama.sh upgrade-locas.sh; do
  case "$f" in
    firstboot.sh) target=locas-firstboot ;;
    locas-app-worker.sh) target=locas-app-worker ;;
    install-ollama.sh) target=locas-install-ollama ;;
    install-open-webui.sh) target=locas-install-open-webui ;;
    repair-ollama.sh) target=locas-repair-ollama ;;
    upgrade-locas.sh) target=locas-upgrade ;;
  esac
  install -m 0755 "packaging/wsl/$f" "$stage/usr/local/sbin/$target"
done
for f in locas-firstboot.service locas-agent.service locas-core.service locas-app-worker.service locas-app-worker.path locas-update-worker.service locas-update-worker.path; do
  install -m 0644 "packaging/wsl/$f" "$stage/etc/systemd/system/$f"
done
tar -C "$stage" -cf dist/native/locasos-native-payload-amd64.tar opt usr etc
(cd dist/native && sha256sum locasos-native-payload-amd64.tar > locasos-native-payload-amd64.tar.sha256)
printf 'Built native application payload (NOT an OS image): %s\n' "$repo/dist/native/locasos-native-payload-amd64.tar"
