#!/usr/bin/env bash
# Phase 1: provision LocasOS in an EXISTING, disposable Ubuntu VM.
# Does not partition disks, create users, or modify host bootloader.
set -Eeuo pipefail
umask 027
if [[ "${EUID}" -ne 0 ]]; then echo 'Run as root inside the VM' >&2; exit 1; fi
if [[ "${LOCAS_NATIVE_VM_ACK:-}" != 'I_UNDERSTAND_THIS_IS_A_DISPOSABLE_VM' ]]; then
  echo 'Requires LOCAS_NATIVE_VM_ACK=I_UNDERSTAND_THIS_IS_A_DISPOSABLE_VM' >&2; exit 1
fi
if grep -qi microsoft /proc/sys/kernel/osrelease; then
  echo 'Refusing WSL; use the separate WSL installer' >&2; exit 1
fi
# Never run this prototype on actual hardware, even when an operator supplies the acknowledgement.
if ! systemd-detect-virt --vm --quiet; then
  echo 'Physical host or unrecognized VM detected; physical installation is forbidden' >&2; exit 1
fi
if [[ ! -f /etc/os-release ]] || ! grep -q '^ID=ubuntu$' /etc/os-release || ! grep -q '^VERSION_ID="24.04"$' /etc/os-release; then
  echo 'Requires Ubuntu Server 24.04 VM' >&2; exit 1
fi
if [[ ! -d /run/systemd/system ]]; then echo 'systemd PID 1 required' >&2; exit 1; fi
payload="${1:-}"
if [[ "${payload##*/}" != 'locasos-native-payload-amd64.tar' ]]; then
 echo 'Refusing an archive name other than the publisher-signed native payload filename' >&2; exit 1
fi
if [[ -z "$payload" || ! -f "$payload" ]]; then echo 'Usage: install-native-vm.sh PAYLOAD.tar' >&2; exit 1; fi
checksum="${payload}.sha256"
if [[ ! -f "$checksum" ]]; then echo 'Missing matching .sha256' >&2; exit 1; fi
# Verify checksum against the named archive, without trusting arbitrary paths in checksum file.
expected="$(awk 'NR == 1 {print $1}' "$checksum")"
[[ "$expected" =~ ^[a-fA-F0-9]{64}$ ]] || { echo 'Invalid checksum' >&2; exit 1; }
actual="$(sha256sum "$payload" | awk '{print $1}')"
[[ "$actual" == "$expected" ]] || { echo 'Payload checksum mismatch' >&2; exit 1; }
# Publisher authentication is mandatory, not merely a matching checksum.
# Verifier binary and publisher public key MUST be obtained independently of the archive.
if [[ -z "${LOCAS_NATIVE_VERIFY_BIN:-}" || ! -x "${LOCAS_NATIVE_VERIFY_BIN:-/nonexistent}" ]]; then
 echo 'Set LOCAS_NATIVE_VERIFY_BIN to the independently obtained trusted locas-release verifier' >&2; exit 1
fi
if [[ -z "${LOCAS_UPDATE_PUBLIC_KEY_B64:-}" || -z "${LOCAS_NATIVE_VERSION:-}" ]]; then
 echo 'Set independently trusted LOCAS_UPDATE_PUBLIC_KEY_B64 and LOCAS_NATIVE_VERSION' >&2; exit 1
fi
"$LOCAS_NATIVE_VERIFY_BIN" verify-native "$LOCAS_NATIVE_VERSION" "$(dirname "$payload")"
# Validate archive types and paths: never unpack symlinks or device nodes as root.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 "$script_dir/verify-archive.py" "$payload"
# First prototype only. Real bare-metal installation requires signed image verification
# BEFORE trusting payload contents and does not use this VM-only script.
for user in locas-core locas-agent locas-ollama; do
  if ! id "$user" >/dev/null 2>&1; then
    useradd --system --create-home --shell /usr/sbin/nologin "$user"
  fi
done
tar -C / -xf "$payload" --no-same-owner
systemctl daemon-reload
systemctl enable locas-firstboot.service locas-agent.service locas-core.service \
  locas-app-worker.path locas-update-worker.path
systemctl start locas-firstboot.service
systemctl start locas-agent.service
systemctl start locas-core.service
systemctl start locas-app-worker.path locas-update-worker.path
systemctl is-active --quiet locas-agent.service locas-core.service
curl --fail --silent --show-error --max-time 5 http://127.0.0.1:59420/healthz | grep -q '"status":"ok"'
echo 'PASS: native disposable VM LocasOS agent/core and localhost health'
