#!/usr/bin/env bash
# Launch a disposable UEFI VM from an operator-supplied Ubuntu Server 24.04 ISO.
# Interactive Ubuntu installation; this script NEVER touches host disks.
set -Eeuo pipefail
umask 077
usage() {
  echo 'Usage: launch-uefi-vm.sh ISO WORKSPACE | --resume WORKSPACE' >&2
  exit 2
}
[[ "$#" -eq 2 ]] || usage
resume=false
iso=
workspace="$2"
if [[ "$1" == "--resume" ]]; then
  resume=true
else
  iso="$(realpath -e -- "$1")"
  [[ -f "$iso" ]] || usage
  case "$(basename "$iso")" in
    ubuntu-24.04*live-server-amd64.iso) ;;
    *) echo 'Expected explicitly supplied Ubuntu 24.04 amd64 live-server ISO' >&2; exit 1 ;;
  esac
fi
for command in qemu-system-x86_64 qemu-img cp; do
  command -v "$command" >/dev/null || { echo "Missing dependency: $command" >&2; exit 1; }
done
find_ovmf() {
  for prefix in /usr/share/OVMF /usr/share/ovmf; do
    for name in OVMF_CODE_4M.fd OVMF_CODE.fd; do
      [[ -f "$prefix/$name" ]] && { echo "$prefix/$name"; return; }
    done
  done
  return 1
}
code="$(find_ovmf)" || { echo 'Install ovmf; OVMF firmware not found' >&2; exit 1; }
vars="${code/CODE/VARS}"
[[ -f "$vars" ]] || { echo "Missing matching OVMF variable template: $vars" >&2; exit 1; }
if [[ "$resume" == false && -e "$workspace" ]] && [[ -n "$(find "$workspace" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  echo "Workspace not empty: $workspace. Refusing to modify previous VM." >&2
  exit 1
fi
if [[ "$resume" == false ]]; then
  mkdir -p -- "$workspace"
fi
workspace="$(realpath -e -- "$workspace")"
if [[ "$resume" == false ]]; then
  cp "$vars" "$workspace/OVMF_VARS.fd"
  qemu-img create -f qcow2 "$workspace/locasos-native-test.qcow2" 40G
else
  [[ -f "$workspace/locasos-native-test.qcow2" && -f "$workspace/OVMF_VARS.fd" ]] || {
    echo 'Existing disposable VM missing from workspace' >&2; exit 1
  }
fi
accel=tcg
[[ -r /dev/kvm && -w /dev/kvm ]] && accel=kvm
echo "Starting isolated UEFI VM with $accel; only the disposable virtual disk will be installed."
echo "Select the 40 GiB virtio disk in the Ubuntu installer. No host drive is attached."
qemu-system-x86_64 -name LocasOS-Native-Disposable \
  -machine q35,accel="$accel" -m 4096 -smp 4 \
  -drive if=pflash,format=raw,readonly=on,file="$code" \
  -drive if=pflash,format=raw,file="$workspace/OVMF_VARS.fd" \
  -drive file="$workspace/locasos-native-test.qcow2",format=qcow2,if=virtio \
  -drive media=cdrom,file="$iso",readonly=on \
  -boot order=d -nic user,model=virtio-net-pci \
  -display gtk
