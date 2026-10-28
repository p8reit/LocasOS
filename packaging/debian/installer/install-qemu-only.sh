#!/bin/bash
# DEVELOPER / QEMU ONLY. Irreversible writes exclusively to one virtio scratch disk.
set -Eeuo pipefail
fail(){ echo "LOCASOS_VM_INSTALL_FAIL: $*" > /dev/ttyS0; exit 1; }
mark(){ echo "LOCASOS_VM_INSTALL_$1" > /dev/ttyS0; }
trap 'fail "unexpected command failure at line $LINENO"' ERR
[[ $(id -u) -eq 0 ]] || fail "root required"
[[ -f /etc/locasos-release ]] || fail "not a LocasOS live image"
grep -q 'EDITION="native-developer-preview"' /etc/locasos-release || fail "wrong edition"
[[ -r /sys/firmware/qemu_fw_cfg/by_name/opt/locasos/dev-install/raw ]] || fail "QEMU opt-in marker missing"
[[ $(cat /sys/firmware/qemu_fw_cfg/by_name/opt/locasos/dev-install/raw) == 'LOCASOS_DISPOSABLE_VM_INSTALL_V1' ]] || fail "invalid marker"
grep -Eiq 'qemu|kvm' /sys/class/dmi/id/sys_vendor || fail "QEMU system vendor required"
[[ -e /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_locasos-scratch ]] || fail "scratch disk identity missing"
disk="$(readlink -f /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_locasos-scratch)"
[[ "$disk" == /dev/sd? ]] || fail "unexpected scratch disk path"
[[ $(lsblk -dn -o TYPE "$disk" | xargs) == disk ]] || fail "not whole disk"
[[ $(lsblk -dn -o RO "$disk" | xargs) == 0 ]] || fail "read-only disk"
bytes="$(blockdev --getsize64 "$disk")"
[[ "$bytes" -ge 68719476736 && "$bytes" -le 137438953472 ]] || fail "scratch disk not 64-128GiB"
[[ $(lsblk -ln -o NAME "$disk" | wc -l) -eq 1 ]] || fail "disk contains partitions"
[[ -z $(lsblk -nr -o FSTYPE "$disk" | tr -d '[:space:]') ]] || fail "existing filesystem"
[[ -z $(lsblk -nr -o MOUNTPOINTS "$disk" | tr -d '[:space:]') ]] || fail "mounted disk"
[[ $(lsblk -dn -o MODEL "$disk" | xargs) == 'QEMU HARDDISK' ]] || fail "unexpected model"
for binary in sgdisk mkfs.vfat mkfs.ext4 rsync grub-install blkid python3; do command -v "$binary" >/dev/null || fail "missing $binary"; done
# Reaffirm exact DMI, fw_cfg, single QEMU disk serial and blank topology
# immediately before the first destructive command.
verified="$(python3 /usr/local/libexec/locasos-vm-target-check)" || fail "VM-only runtime device preflight refused"
[[ "$verified" == "$disk" ]] || fail "VM target changed after preflight"
mark 'STARTED'
sgdisk --clear "$disk"
sgdisk --new=1:1MiB:+1GiB --typecode=1:ef00 --change-name=1:EFI "$disk"
sgdisk --new=2:0:+12GiB --typecode=2:8300 --change-name=2:ROOT_A "$disk"
sgdisk --new=3:0:+12GiB --typecode=3:8300 --change-name=3:ROOT_B "$disk"
sgdisk --new=4:0:+4GiB --typecode=4:8300 --change-name=4:RECOVERY "$disk"
sgdisk --new=5:0:0 --typecode=5:8300 --change-name=5:DATA "$disk"
partprobe "$disk"; udevadm settle
mark "PARTITIONED"
esp="${disk}1"; root="${disk}2"; backup="${disk}3"; recovery="${disk}4"; data="${disk}5"
for path in "$esp" "$root" "$backup" "$recovery" "$data"; do [[ -b "$path" ]] || fail "partition missing: $path"; done
mkfs.vfat -F 32 "$esp"
mkfs.ext4 -F -q "$root"
mkfs.ext4 -F -q "$backup"
mkfs.ext4 -F -q "$recovery"
mkfs.ext4 -F -q "$data"
mkdir -p /mnt/locasos-target
mount "$root" /mnt/locasos-target
trap 'umount -R /mnt/locasos-target || true' EXIT
# Copy live runtime contents; never include runtime filesystems or live overlay backing.
rsync -aHAX --numeric-ids --one-file-system --exclude=/proc/*** --exclude=/sys/*** \
 --exclude=/dev/*** --exclude=/run/*** --exclude=/tmp/*** \
 --exclude=/mnt/*** --exclude=/media/*** --exclude=/live/*** \
 / /mnt/locasos-target/
mark "ROOT_COPIED"
mkdir -p /mnt/locasos-target/{dev,proc,sys,run,boot/efi,var/lib/locas}
mount "$esp" /mnt/locasos-target/boot/efi
uuid="$(blkid -s UUID -o value "$root")"
efi_uuid="$(blkid -s UUID -o value "$esp")"
data_uuid="$(blkid -s UUID -o value "$data")"
cat > /mnt/locasos-target/etc/fstab <<EOF
UUID=$uuid / ext4 defaults 0 1
UUID=$efi_uuid /boot/efi vfat umask=0077 0 2
UUID=$data_uuid /var/lib/locas ext4 defaults 0 2
EOF
# The installed-disk readiness service checks this data-volume sentinel,
# proving the independently mounted persistent partition survived reboot.
mount "$data" /mnt/locasos-target/var/lib/locas
printf 'locasos-vm-install-v1\n' > /mnt/locasos-target/var/lib/locas/.locasos-vm-install-sentinel
for d in dev proc sys run; do mount --bind "/$d" "/mnt/locasos-target/$d"; done
# UEFI removable path avoids modifying host EFI variables, even in QEMU.
chroot /mnt/locasos-target grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=LocasOS --removable --no-nvram --recheck || fail "grub-install failed"
chroot /mnt/locasos-target update-grub || fail "update-grub failed"
mark "EFI_CONFIGURED"
touch /mnt/locasos-target/etc/locasos-installed-vm-test
sync
mark 'COMPLETE'
