#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/../.."

# Destructive upgrade/rollback simulation runs only inside an ephemeral Docker
# root filesystem. The host and any user's WSL installation remain untouched.
docker run --rm --entrypoint /bin/bash \
  -v "$PWD/dist:/update-test:ro" locasos-rootfs-build:dev \
  -euxo pipefail -c '
    mkdir -p /tmp/fakebin /var/lib/locas-ollama/models
    printf "#!/bin/sh\nexit 0\n" > /tmp/fakebin/systemctl
    printf "#!/bin/sh\nexit 1\n" > /tmp/fakebin/curl
    printf "#!/bin/sh\nprintf \"1\\n\"\n" > /tmp/fakebin/seq
    printf "#!/bin/sh\nexit 0\n" > /tmp/fakebin/journalctl
    chmod +x /tmp/fakebin/*
    export PATH="/tmp/fakebin:$PATH"

    # A deliberately different previous install verifies actual restoration.
    printf "previous-core\n" > /opt/locas/bin/locas-core
    printf "previous-agent\n" > /opt/locas/bin/locas-agent
    printf "previous-worker\n" > /etc/systemd/system/locas-update-worker.service
    rm -f /usr/local/sbin/locas-repair-ollama
    printf "PRESERVE_MODELS\n" > /var/lib/locas-ollama/models/preserve
    cp /etc/systemd/system/locas-update-worker.service /tmp/old-worker

    # Force post-extraction health to fail. The script must fail and recover.
    if /bin/bash /usr/local/sbin/locas-upgrade \
      /update-test/locasos-upgrade-amd64.tar; then
        echo "Upgrade unexpectedly succeeded without healthy services" >&2
        exit 1
    fi
    test "$(cat /opt/locas/bin/locas-core)" = "previous-core"
    test "$(cat /opt/locas/bin/locas-agent)" = "previous-agent"
    cmp /tmp/old-worker /etc/systemd/system/locas-update-worker.service
    test ! -e /usr/local/sbin/locas-repair-ollama
    test "$(cat /var/lib/locas-ollama/models/preserve)" = "PRESERVE_MODELS"
    test -n "$(find /var/lib/locas-upgrades -name "preupgrade-*.tar" -type f -print -quit)"
    echo "PASS: failed upgrade restored old binaries and services, removed additions and preserved model data"
  '
