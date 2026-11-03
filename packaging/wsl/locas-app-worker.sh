#!/usr/bin/env bash
# Compatibility entry point; all privileged queue processing is typed and audited.
set -Eeuo pipefail
exec /opt/locas/bin/locas-app-worker
