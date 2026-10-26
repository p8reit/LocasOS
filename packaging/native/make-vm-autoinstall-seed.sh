#!/usr/bin/env bash
# Generate NoCloud autoinstall seed for a disposable QEMU disk only.
# No host disks are created/modified by this script.
set -Eeuo pipefail
umask 077
if [[ "$#" != 2 ]]; then
  echo 'Usage: make-vm-autoinstall-seed.sh EMPTY-WORKSPACE SSH-PUBLIC-KEY-FILE' >&2; exit 2
fi
if [[ "${LOCAS_NATIVE_VM_AUTOINSTALL_ACK:-}" != "I_ACCEPT_ERASING_A_DISPOSABLE_QEMU_IMAGE_ONLY" ]]; then
  echo 'Explicit disposable QEMU image acknowledgment required' >&2; exit 1
fi
workspace="$1"
pubkey="$2"
for command in cloud-localds openssl python3; do
  command -v "$command" >/dev/null || { echo "Missing $command" >&2; exit 1; }
done
[[ -f "$pubkey" ]] || { echo 'SSH public key not found' >&2; exit 1; }
if [[ -e "$workspace" && -n "$(find "$workspace" -mindepth 1 -print -quit)" ]]; then
  echo 'Workspace must be empty; refusing to overwrite data' >&2; exit 1
fi
mkdir -p -- "$workspace"
workspace="$(realpath -e -- "$workspace")"
read -r key_type key_data _ < "$pubkey" || true
key="$key_type $key_data"
[[ "$key_type" =~ ^ssh-(ed25519|rsa)$ && "$key_data" =~ ^[A-Za-z0-9+/=]+$ ]] || {
  echo 'Expected a valid-looking OpenSSH public key' >&2; exit 1
}
# Password is intentionally random and never logged. SSH key access only.
password="$(openssl rand -base64 48)"
hash="$(printf %s "$password" | openssl passwd -6 -stdin)"
printf %s "$password" > "$workspace/vm-test-password"
chmod 0600 "$workspace/vm-test-password"
unset password
cat > "$workspace/user-data" <<EOF
#cloud-config
autoinstall:
  version: 1
  interactive-sections: []
  refresh-installer:
    update: false
  early-commands:
    # Both virtualization and the unique disposable disk identity must match.
    - 'test "\$(systemd-detect-virt)" = qemu || test "\$(systemd-detect-virt)" = kvm'
    - 'test "\$(lsblk -dn -o TYPE | grep -c "^disk$")" -eq 1'
    - 'lsblk -dn -o SERIAL | grep -Fxq "LOCASOS_VM_ONLY_001"'
  identity:
    hostname: locasos-test
    username: locasdev
    password: '$hash'
  ssh:
    install-server: true
    allow-pw: false
    authorized-keys:
      - '$key'
  storage:
    layout:
      name: direct
      match:
        serial: LOCASOS_VM_ONLY_001
  shutdown: poweroff
EOF
printf 'instance-id: locasos-uefi-test-001\nlocal-hostname: locasos-test\n' > "$workspace/meta-data"
cloud-localds "$workspace/seed.iso" "$workspace/user-data" "$workspace/meta-data"
chmod 0600 "$workspace/user-data" "$workspace/meta-data" "$workspace/seed.iso"
echo "Prepared disposable VM-only seed in $workspace"
echo 'Requires a QEMU disk exposing the exact serial LOCASOS_VM_ONLY_001.'
echo 'Do NOT pass real block devices to the VM. This seed will erase ONLY the matching disposable virtual disk.'
