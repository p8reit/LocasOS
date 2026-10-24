#!/usr/bin/env bash
set -Eeuo pipefail
archive="${1:-dist/native/locasos-native-payload-amd64.tar}"
test -s "$archive"
test -s "${archive}.sha256"
expected="$(awk 'NR==1 {print $1}' "${archive}.sha256")"
actual="$(sha256sum "$archive" | awk '{print $1}')"
test "$expected" = "$actual"
for member in opt/locas/bin/locas-core opt/locas/bin/locas-agent opt/locas/bin/locas-updater \
  etc/systemd/system/locas-firstboot.service etc/systemd/system/locas-agent.service \
  etc/systemd/system/locas-core.service usr/local/sbin/locas-firstboot; do
  tar -tf "$archive" | grep -Fxq "$member" || { echo "Missing $member" >&2; exit 1; }
done
python3 "$(dirname "$0")/verify-archive.py" "$archive"
echo "PASS: native VM payload checksum and mandatory contents"
