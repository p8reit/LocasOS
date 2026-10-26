#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
mkdir -p build dist
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
version="$(git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)"
go build -trimpath -ldflags="-s -w -X main.buildVersion=${version}" -o build/locas-core ./cmd/locas-core
go build -trimpath -ldflags="-s -w" -o build/locas-agent ./cmd/locas-agent
go build -trimpath -ldflags="-s -w -X main.trustedPublicKeyBase64=${LOCAS_UPDATE_PUBLIC_KEY_B64:-}" -o build/locas-updater ./cmd/locas-updater
tag="locasos-rootfs-build:dev"
docker build -f packaging/wsl/rootfs.Dockerfile -t "$tag" .
container="$(docker create "$tag")"
trap 'docker rm -f "$container" >/dev/null 2>&1 || true' EXIT
docker export "$container" -o dist/locasos-wsl-amd64.tar
(cd dist && sha256sum locasos-wsl-amd64.tar > locasos-wsl-amd64.tar.sha256)
echo "Built dist/locasos-wsl-amd64.tar"

# Build a small upgrade bundle independent of the rootfs so existing models,
# account state and app data are not replaced during development upgrades.
stage="$(mktemp -d)"
trap 'rm -rf "$stage"; docker rm -f "$container" >/dev/null 2>&1 || true' EXIT
mkdir -p "$stage/opt/locas/bin" "$stage/etc/systemd/system" "$stage/usr/local/sbin"
cp build/locas-core build/locas-agent build/locas-updater "$stage/opt/locas/bin/"
printf 'debian:13\n' > "$stage/opt/locas/bin/locas-os-base"
cp packaging/wsl/locas-core.service packaging/wsl/locas-agent.service \
   packaging/wsl/locas-app-worker.path packaging/wsl/locas-app-worker.service \
   "$stage/etc/systemd/system/"
cp packaging/wsl/locas-app-worker.sh "$stage/usr/local/sbin/locas-app-worker"
cp packaging/wsl/install-ollama.sh "$stage/usr/local/sbin/locas-install-ollama"
cp packaging/wsl/install-open-webui.sh "$stage/usr/local/sbin/locas-install-open-webui"
cp packaging/wsl/repair-ollama.sh "$stage/usr/local/sbin/locas-repair-ollama"
cp packaging/wsl/upgrade-locas.sh "$stage/usr/local/sbin/locas-upgrade"
cp packaging/wsl/locas-update-worker.service packaging/wsl/locas-update-worker.path "$stage/etc/systemd/system/"
chmod 0755 "$stage/opt/locas/bin/"* "$stage/usr/local/sbin/"*
tar -C "$stage" -cf dist/locasos-upgrade-amd64.tar opt etc usr
(cd dist && sha256sum locasos-upgrade-amd64.tar > locasos-upgrade-amd64.tar.sha256)
echo "Built dist/locasos-upgrade-amd64.tar"
