#!/usr/bin/env bash
# Disposable QEMU autoinstall. Never attaches physical disks.
set -Eeuo pipefail
umask 077
if [[ "$#" -ne 3 ]]; then
  echo 'Usage: launch-vm-autoinstall.sh VERIFIED-UBUNTU-ISO SEED.iso EMPTY-DISK-WORKSPACE' >&2; exit 2
fi
if [[ "${LOCAS_NATIVE_VM_AUTOINSTALL_ACK:-}" != 'I_ACCEPT_ERASING_A_DISPOSABLE_QEMU_IMAGE_ONLY' ]]; then
  echo 'Operator must explicitly accept disposal of a newly created QEMU virtual disk' >&2; exit 1
fi
iso="$(realpath -e -- "$1")"
seed="$(realpath -e -- "$2")"
workspace="$3"
case "$(basename "$iso")" in
  ubuntu-24.04*live-server-amd64.iso|locasos-native-uefi-vm-experimental.iso) ;;
  *) echo 'Only Ubuntu 24.04 AMD64 or experimental LocasOS VM ISO is supported' >&2; exit 1 ;;
esac
[[ "$(basename "$seed")" == seed.iso ]] || { echo 'Expected separately provisioned seed.iso' >&2; exit 1; }
if [[ -e "$workspace" ]] && [[ -n "$(find "$workspace" -mindepth 1 -print -quit)" ]]; then
  echo "Refusing nonempty workspace: $workspace" >&2; exit 1
fi
for binary in qemu-system-x86_64 qemu-img xorriso systemd-detect-virt; do
  command -v "$binary" >/dev/null || { echo "Missing dependency: $binary" >&2; exit 1; }
done
[[ -f /usr/share/OVMF/OVMF_CODE_4M.fd && -f /usr/share/OVMF/OVMF_VARS_4M.fd ]] || {
  echo 'OVMF 4M UEFI firmware required' >&2; exit 1
}
mkdir -p -- "$workspace"
workspace="$(realpath -e -- "$workspace")"
xorriso -osirrox on -indev "$iso" -extract /casper/vmlinuz "$workspace/vmlinuz" -extract /casper/initrd "$workspace/initrd"
qemu-img create -f qcow2 "$workspace/disposable.qcow2" 40G
cp /usr/share/OVMF/OVMF_VARS_4M.fd "$workspace/ovmf-vars.fd"
accel=tcg
[[ -r /dev/kvm && -w /dev/kvm ]] && accel=kvm
echo "Starting QEMU autoinstall. Only newly created $workspace/disposable.qcow2 can be erased."
echo 'Installer early-commands also require QEMU and exact serial LOCASOS_VM_ONLY_001.'
qemu-system-x86_64 -machine "q35,accel=$accel" -m 4096 -smp 4 -no-reboot \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
  -drive if=pflash,format=raw,file="$workspace/ovmf-vars.fd" \
  -drive if=none,id=locasdisk,format=qcow2,file="$workspace/disposable.qcow2" \
  -device virtio-blk-pci,drive=locasdisk,serial=LOCASOS_VM_ONLY_001 \
  -drive media=cdrom,format=raw,readonly=on,file="$iso" \
  -drive media=cdrom,format=raw,readonly=on,file="$seed" \
  -kernel "$workspace/vmlinuz" -initrd "$workspace/initrd" \
  -append 'autoinstall ds=nocloud console=ttyS0,115200n8' \
  -nic user,model=virtio-net-pci \
  -display none -serial mon:stdio
echo 'QEMU stopped; inspect guest logs and disk before starting a disk-only boot.'
