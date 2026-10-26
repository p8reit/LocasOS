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
gpgv --keyring "$keyring" "$signature" "$sums"
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
