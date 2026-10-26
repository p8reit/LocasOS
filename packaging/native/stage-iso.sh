#!/usr/bin/env bash
# Development-only: preserve Ubuntu boot catalog while adding read-only LocasOS files.
# This is a remastered Ubuntu installer, NOT an approved standalone LocasOS OS release.
set -Eeuo pipefail
if [[ "$#" -ne 6 ]]; then
 echo 'Usage: stage-iso.sh BASE-ISO SHA256SUMS SHA256SUMS.gpg TRUSTED-KEYRING NATIVE-PAYLOAD.tar OUTPUT.iso' >&2; exit 2
fi
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"$repo/packaging/native/verify-ubuntu-iso.sh" "$1" "$2" "$3" "$4"
payload="$(realpath -e -- "$5")"
output="$6"
[[ "$output" == *.iso ]] || { echo 'Output must be an ISO filename' >&2; exit 1; }
[[ ! -e "$output" ]] || { echo 'Refusing to overwrite existing ISO' >&2; exit 1; }
command -v xorriso >/dev/null || { echo 'Install xorriso' >&2; exit 1; }
command -v python3 >/dev/null || exit 1
[[ -s "${payload}.sha256" ]] || { echo 'Missing payload checksum' >&2; exit 1; }
bash "$repo/packaging/native/validate-payload.sh" "$payload"
if [[ -z "${LOCAS_NATIVE_VERIFY_BIN:-}" || ! -x "${LOCAS_NATIVE_VERIFY_BIN:-/nonexistent}" ||
      -z "${LOCAS_NATIVE_VERSION:-}" || -z "${LOCAS_UPDATE_PUBLIC_KEY_B64:-}" ]]; then
  echo 'Native signing requires trusted verifier, public key and exact version' >&2; exit 1
fi
"$LOCAS_NATIVE_VERIFY_BIN" verify-native "$LOCAS_NATIVE_VERSION" "$(dirname "$payload")"
test -s "$(dirname "$payload")/locasos-native.json"
test -s "$(dirname "$payload")/locasos-native.sig"
# Only embed a VM prototype, not execution hooks or autoinstall scripts.
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
install -m 0644 "$payload" "$stage/locasos-native-payload-amd64.tar"
install -m 0644 "${payload}.sha256" "$stage/locasos-native-payload-amd64.tar.sha256"
install -m 0644 "$(dirname "$payload")/locasos-native.json" "$stage/locasos-native.json"
install -m 0644 "$(dirname "$payload")/locasos-native.sig" "$stage/locasos-native.sig"
cat > "$stage/README.txt" <<'END'
LocasOS experimental VM-only native payload.
The installer is the unmodified interactive Ubuntu Server installer.
It does not automatically install LocasOS applications or partition any disks.
The payload has a publisher signature. Verify independently before deploying.
Do not use this ISO on physical hardware. No host disk auto-install is configured.
END
# Replay the original boot configuration from the authenticated ISO.
# Reject if xorriso cannot preserve the source boot image metadata.
xorriso -indev "$1" -outdev "$output" \
  -map "$stage" /locasos \
  -boot_image any replay \
  -commit -end
test -s "$output" || { echo 'ISO build produced empty file' >&2; exit 1; }
sha256sum "$output" > "${output}.sha256"
echo "Built experimental Ubuntu UEFI VM ISO: $output (NOT a native signed release)"
