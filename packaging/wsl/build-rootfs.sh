#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
mkdir -p build dist
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o build/locas-core ./cmd/locas-core
go build -trimpath -ldflags="-s -w" -o build/locas-agent ./cmd/locas-agent
tag="locasos-rootfs-build:dev"
docker build -f packaging/wsl/rootfs.Dockerfile -t "$tag" .
container="$(docker create "$tag")"
trap 'docker rm -f "$container" >/dev/null 2>&1 || true' EXIT
docker export "$container" -o dist/locasos-wsl-amd64.tar
(cd dist && sha256sum locasos-wsl-amd64.tar > locasos-wsl-amd64.tar.sha256)
echo "Built dist/locasos-wsl-amd64.tar"
