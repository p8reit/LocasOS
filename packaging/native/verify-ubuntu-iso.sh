#!/usr/bin/env bash
# Verify Ubuntu ISO with an operator-trusted Ubuntu signing keyring.
set -Eeuo pipefail
if [[ "$#" -ne 4 ]]; then
 echo 'Usage: verify-ubuntu-iso.sh ISO SHA256SUMS SHA256SUMS.gpg TRUSTED-KEYRING.gpg' >&2; exit 2
fi
iso="$(realpath -e -- "$1")"
sums="$(realpath -e -- "$2")"
signature="$(realpath -e -- "$3")"
keyring="$(realpath -e -- "$4")"
[[ -f "$iso" && -f "$sums" && -f "$signature" && -f "$keyring" ]] || exit 1
case "$(basename "$iso")" in
  ubuntu-24.04*live-server-amd64.iso) ;;
  *) echo 'Refusing non-24.04 Ubuntu Server AMD64 image' >&2; exit 1 ;;
esac
command -v gpgv >/dev/null || { echo 'Missing gpgv' >&2; exit 1; }
# NEVER auto-download a key from the same source as the ISO or accept an untrusted key.
# The official Ubuntu CD Image Automatic Signing Key (2012), published by Canonical.
# Do not accept unrelated archive-signing keys from the same keyring.
expected_fpr="${LOCAS_UBUNTU_CD_IMAGE_FINGERPRINT:-843938DF228D22F7B3742BC0D94AA3F0EFE21092}"
[[ "$expected_fpr" =~ ^[0-9A-F]{40}$ ]] || { echo 'Invalid expected signer fingerprint' >&2; exit 1; }
status="$(gpgv --status-fd 1 --keyring "$keyring" "$signature" "$sums" 2>&1)" || {
  printf '%s\n' "$status" >&2; exit 1
}
if ! printf '%s\n' "$status" | awk -v fpr="$expected_fpr" '$2=="VALIDSIG" && $3==fpr {found=1} END{exit !found}'; then
  echo "Signed manifest not authenticated by pinned Ubuntu CD key: $expected_fpr" >&2
  exit 1
fi
# Ubuntu SHA256SUMS is a text manifest. Match the exact basename once.
digest="$(awk -v name="$(basename "$iso")" '
  {
    sha=$1; filename=$2
    sub(/^\*/, "", filename)
    sub(/^\.\//, "", filename)
    if (filename == name) { print sha; count++ }
  }
  END { if (count != 1) exit 1 }
' "$sums")"
[[ "$digest" =~ ^[a-fA-F0-9]{64}$ ]] || { echo 'Invalid SHA256 digest' >&2; exit 1; }
actual="$(sha256sum "$iso" | awk '{print $1}')"
[[ "${digest,,}" == "${actual,,}" ]] || { echo 'ISO SHA256 mismatch' >&2; exit 1; }
printf 'PASS: authentic Ubuntu manifest signature and exact ISO SHA256 verified: %s\n' "$(basename "$iso")"
