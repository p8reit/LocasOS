#!/usr/bin/env bash
# Destructive test installer. ONLY for a disposable QEMU virtio disk, never physical devices.
set -Eeuo pipefail
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
die() { echo "LOCASOS_VM_INSTALL_REFUSED: $*" >&2; exit 2; }
[[ "${EUID}" -eq 0 ]] || die root_required
[[ -f /etc/locasos-release ]] || die not_locasos_live
grep -q 'EDITION="native-developer-preview"' /etc/locasos-release || die not_live_preview
[[ -d /run/live/medium ]] || die not_live_media
[[ "$(cat /sys/class/dmi/id/product_name 2>/dev/null)" == "LocasOS-QEMU-InstallTest" ]] || die qemu_product_marker_missing
[[ "$(cat /sys/class/dmi/id/sys_vendor 2>/dev/null)" == "QEMU" ]] || die qemu_vendor_missing
[[ -f /sys/firmware/qemu_fw_cfg/by_name/opt/locasos/ephemeral/raw ]] || die fw_cfg_marker_missing
[[ "$(cat /sys/firmware/qemu_fw_cfg/by_name/opt/locasos/ephemeral/raw)" == "locasos-ephemeral-disk-v1" ]] || die invalid_fw_cfg_marker
target=/dev/vda
[[ -b "$target" ]] || die missing_virtio_blk
[[ "$(lsblk -dn -o TYPE "$target")" == disk ]] || die not_disk
[[ "$(lsblk -dn -o TRAN "$target" | tr -d ' ')" == virtio || -e /sys/block/vda/device ]] || die not_virtio_disk
[[ "$(lsblk -dn -o RO "$target" | tr -d ' ')" == 0 ]] || die read_only
[[ -z "$(lsblk -nr -o MOUNTPOINTS "$target" | tr -d '[:space:]')" ]] || die mounted_target
[[ -z "$(lsblk -nr -o FSTYPE "$target" | tr -d '[:space:]')" ]] || die contains_filesystem
[[ -z "$(lsblk -nr -o NAME "$target" | tail -n +2)" ]] || die has_existing_partitions
size="$(blockdev --getsize64 "$target")"
((size >= 64*1024*1024*1024)) || die disk_too_small
[[ -z "$(lsblk -nr -o NAME /dev/vda | tail -n +2)" ]] || die disk_has_children
sourcefs="$(find /run/live/medium/live -maxdepth 1 -name 'filesystem.squashfs' -type f -print -quit)"
[[ -n "$sourcefs" ]] || die missing_live_squashfs
for cmd in sgdisk mkfs.vfat mkfs.ext4 unsquashfs blkid grub-install partprobe udevadm; do command -v "$cmd" >/dev/null || die "missing_tool_$cmd"; done
# Reaffirm never touching any disk except a private virtio-blk fixture, after all checks.
echo 'LOCASOS_VM_INSTALL_BEGIN' >/dev/ttyS0
sgdisk --zap-all "$target"
sgdisk -n 1:1MiB:+1GiB -t 1:ef00 -c 1:LOCAS_EFI "$target"
sgdisk -n 2:0:+12GiB -t 2:8300 -c 2:LOCAS_ROOT_A "$target"
sgdisk -n 3:0:+12GiB -t 3:8300 -c 3:LOCAS_ROOT_B "$target"
sgdisk -n 4:0:+4GiB -t 4:8300 -c 4:LOCAS_RECOVERY "$target"
sgdisk -n 5:0:0 -t 5:8300 -c 5:LOCAS_DATA "$target"
partprobe "$target"
udevadm settle
for part in 1 2 3 4 5; do [[ -b "/dev/vda$part" ]] || die "missing_partition_$part"; done
mkfs.vfat -F 32 -n LOCAS_EFI /dev/vda1
mkfs.ext4 -F -q -L LOCAS_ROOT_A /dev/vda2
mkfs.ext4 -F -q -L LOCAS_ROOT_B /dev/vda3
mkfs.ext4 -F -q -L LOCAS_RECOVERY /dev/vda4
mkfs.ext4 -F -q -L LOCAS_DATA /dev/vda5
root=/mnt/locasos-vm-target
mkdir -p "$root"
mount /dev/vda2 "$root"
cleanup() {
 set +e
 for path in dev/pts dev proc sys run; do umount -l "$root/$path" 2>/dev/null; done
 umount -l "$root/boot/efi" 2>/dev/null
 umount -l "$root/var/lib/locasos-data" 2>/dev/null
 umount -l "$root" 2>/dev/null
}
trap cleanup EXIT
# The live rootfs is unpacked offline; no external repositories or system disk mounts.
unsquashfs -no-progress -f -d "$root" "$sourcefs" >/dev/null
mkdir -p "$root/boot/efi" "$root/var/lib/locasos-data" "$root/etc"
mount /dev/vda1 "$root/boot/efi"
mount /dev/vda5 "$root/var/lib/locasos-data"
rootuuid="$(blkid -s UUID -o value /dev/vda2)"
efiuuid="$(blkid -s UUID -o value /dev/vda1)"
datauuid="$(blkid -s UUID -o value /dev/vda5)"
cat > "$root/etc/fstab" <<EOF
UUID=$rootuuid / ext4 defaults 0 1
UUID=$efiuuid /boot/efi vfat umask=0077 0 2
UUID=$datauuid /var/lib/locasos-data ext4 defaults 0 2
EOF
# Remove live-only auto-boot hooks to keep installed system independent.
rm -f "$root/etc/systemd/system/multi-user.target.wants/live-config.service"
for dir in dev dev/pts proc sys run; do mkdir -p "$root/$dir"; done
mount --bind /dev "$root/dev"
mount -t devpts devpts "$root/dev/pts"
mount -t proc proc "$root/proc"
mount -t sysfs sysfs "$root/sys"
mount -t tmpfs tmpfs "$root/run"
[[ -e "$root/vmlinuz" || -n "$(find "$root/boot" -maxdepth 1 -name 'vmlinuz-*' -print -quit)" ]] || die missing_kernel
kernel="$(find "$root/boot" -maxdepth 1 -name 'vmlinuz-*' -printf '%f\n' | sort -V | tail -n 1)"
initrd="initrd.img-${kernel#vmlinuz-}"
[[ -s "$root/boot/$initrd" ]] || die missing_initramfs
# Install UEFI removable bootloader to the *scratch* ESP; never write host EFI variables.
chroot "$root" grub-install --target=x86_64-efi --efi-directory=/boot/efi --boot-directory=/boot --removable --no-nvram --recheck
mkdir -p "$root/boot/grub"
cat > "$root/boot/grub/grub.cfg" <<EOF
set timeout=2
set default=0
menuentry 'LocasOS VM test slot A' {
 linux /boot/$kernel root=UUID=$rootuuid ro console=ttyS0,115200n8
 initrd /boot/$initrd
}
EOF
# On first installed boot, emit a positive systemd readiness marker over serial.
cat > "$root/etc/systemd/system/locasos-vm-installed-ready.service" <<'EOF'
[Unit]
Description=LocasOS disposable VM installed-boot readiness signal
After=multi-user.target locas-core.service
Wants=locas-core.service
[Service]
Type=oneshot
ExecStart=/bin/sh -c 'systemctl is-active --quiet locas-core && curl -fsS --max-time 3 http://127.0.0.1:59420/healthz | grep -q 'locas-core' && printf "LOCASOS_INSTALLED_READY\n" > /dev/ttyS0'
[Install]
WantedBy=multi-user.target
EOF
mkdir -p "$root/etc/systemd/system/multi-user.target.wants"
ln -sf /etc/systemd/system/locasos-vm-installed-ready.service "$root/etc/systemd/system/multi-user.target.wants/locasos-vm-installed-ready.service"
sync
echo 'LOCASOS_VM_INSTALL_COMPLETE' >/dev/ttyS0
