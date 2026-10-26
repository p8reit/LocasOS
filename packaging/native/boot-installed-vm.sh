#!/usr/bin/env bash
# Boot an EXISTING disposable UEFI guest disk following VM-only Ubuntu autoinstall.
set -Eeuo pipefail
umask 077
if [[ "$#" -ne 1 ]]; then echo 'Usage: boot-installed-vm.sh EXISTING-WORKSPACE' >&2; exit 2; fi
if [[ "${LOCAS_NATIVE_VM_AUTOINSTALL_ACK:-}" != "I_ACCEPT_ERASING_A_DISPOSABLE_QEMU_IMAGE_ONLY" ]]; then
 echo 'Operator acknowledgment for disposable VM-only testing required' >&2; exit 1
fi
workspace="$(realpath -e -- "$1")"
disk="$workspace/disposable.qcow2"
vars="$workspace/ovmf-vars.fd"
[[ -f "$disk" && -f "$vars" ]] || { echo 'Existing disposable virtual disk and OVMF variables required' >&2; exit 1; }
if ! qemu-img info --output=json "$disk" | python3 -c 'import json,sys; v=json.load(sys.stdin); assert v["format"]=="qcow2" and v["virtual-size"]==40*1024**3'; then
 echo 'Unexpected disk format or capacity; refusing' >&2; exit 1
fi
accel=tcg
[[ -r /dev/kvm && -w /dev/kvm ]] && accel=kvm
echo 'Booting isolated guest disk, forwarding guest SSH only to host 127.0.0.1:2222.'
qemu-system-x86_64 -machine "q35,accel=$accel" -m 4096 -smp 4 \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
  -drive if=pflash,format=raw,file="$vars" \
  -drive if=none,id=locasdisk,format=qcow2,file="$disk" \
  -device virtio-blk-pci,drive=locasdisk,serial=LOCASOS_VM_ONLY_001 \
  -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:2222-:22 \
  -display none -serial mon:stdio
