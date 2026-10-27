#!/usr/bin/env bash
# LocasOS Debian-based developer live ISO. No disk installer or autoinstall.
set -Eeuo pipefail
umask 027
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"
for tool in lb go sha256sum; do command -v "$tool" >/dev/null || { echo "Missing $tool" >&2; exit 1; }; done
: "${LOCAS_UPDATE_PUBLIC_KEY_B64:?Publisher public key required for native payload}"
bash packaging/native/build-payload.sh
bash packaging/native/validate-payload.sh
workspace="$(mktemp -d)"
trap 'rm -rf "$workspace"' EXIT
mkdir -p "$workspace/config/package-lists" "$workspace/config/includes.chroot/usr/local/share/locasos" "$workspace/config/includes.chroot/usr/local/sbin" "$workspace/config/includes.chroot/etc/systemd/system" "$workspace/config/hooks/live"
# Source-build payload remains explicitly VM-only and is NOT executed by the live image.
install -m 0644 dist/native/locasos-native-payload-amd64.tar "$workspace/config/includes.chroot/usr/local/share/locasos/"
install -m 0644 dist/native/locasos-native-payload-amd64.tar.sha256 "$workspace/config/includes.chroot/usr/local/share/locasos/"
install -m 0755 packaging/debian/installer/install-ephemeral-guest.sh "$workspace/config/includes.chroot/usr/local/sbin/locasos-vm-install"
# Reuse exactly the same baseline system dependencies as WSL.
grep -Ev '^[[:space:]]*(#|$)' packaging/common/base-packages.txt > "$workspace/config/package-lists/locasos.list.chroot"
cat >> "$workspace/config/package-lists/locasos.list.chroot" <<'LIST'
openssh-client
pciutils
python3
parted
gdisk
dosfstools
e2fsprogs
squashfs-tools
grub-efi-amd64-bin
grub2-common
LIST
cat > "$workspace/config/includes.chroot/etc/systemd/system/locasos-vm-live-ready.service" <<'SERVICE'
[Unit]
Description=LocasOS disposable live guest readiness
After=multi-user.target locas-core.service
Wants=locas-core.service
[Service]
Type=oneshot
ExecStart=/bin/sh -c 'systemctl is-active --quiet locas-core && printf "LOCASOS_LIVE_READY\\n" > /dev/ttyS0'
[Install]
WantedBy=multi-user.target
SERVICE
cat > "$workspace/config/includes.chroot/etc/systemd/system/locasos-vm-test-install.service" <<'SERVICE'
[Unit]
Description=LocasOS destructive install exclusively on QEMU marked scratch disk
After=locas-core.service
ConditionPathExists=/sys/firmware/qemu_fw_cfg/by_name/opt/locasos/ephemeral/raw
[Service]
Type=oneshot
ExecStart=/usr/local/sbin/locasos-vm-install
StandardOutput=journal+console
StandardError=journal+console
[Install]
WantedBy=multi-user.target
SERVICE
# Debian Live defaults to a live session, not automatic host disk installation.
cat > "$workspace/config/hooks/live/010-locasos-brand.hook.chroot" <<'HOOK'
#!/bin/sh
set -eu
printf 'LocasOS Development Live\n' > /etc/issue
cat > /etc/locasos-release <<'RELEASE'
NAME="LocasOS"
PRETTY_NAME="LocasOS Developer Live (Debian-based)"
EDITION="native-developer-preview"
RELEASE
HOOK
cat > "$workspace/config/hooks/live/020-locasos-services.hook.chroot" <<'HOOK'
#!/bin/sh
set -eu
# Image-build-time extraction into the disposable rootfs, never a host system.
for user in locas-core locas-agent locas-ollama; do
  if ! id "$user" >/dev/null 2>&1; then
    useradd --system --create-home --shell /usr/sbin/nologin "$user"
  fi
done
tar -C / -xf /usr/local/share/locasos/locasos-native-payload-amd64.tar --no-same-owner
systemctl enable locas-firstboot.service locas-agent.service locas-core.service locasos-vm-live-ready.service locasos-vm-test-install.service
# Path workers are not enabled by default until native lifecycle approval is complete.
HOOK
chmod 0755 "$workspace/config/hooks/live/"*.hook.chroot
cd "$workspace"
lb config --mode debian --distribution trixie --architectures amd64 \
  --binary-images iso-hybrid --linux-flavours amd64 --linux-packages linux-image \
  --debian-installer none --archive-areas "main" \
  --iso-application "LocasOS Developer Live" --iso-publisher "LocasOS Community" \
  --iso-volume "LOCASOS_DEV" --apt-recommends false
lb build
mkdir -p "$repo/dist/debian"
image="$(find . -maxdepth 1 -type f -name '*.iso' -print -quit)"
[[ -n "$image" ]] || { echo 'Live-build did not produce an ISO' >&2; exit 1; }
install -m 0644 "$image" "$repo/dist/debian/locasos-native-developer-amd64.iso"
(cd "$repo/dist/debian" && sha256sum locasos-native-developer-amd64.iso > locasos-native-developer-amd64.iso.sha256)
echo 'Built Debian-based LocasOS developer live ISO; NOT an OS installer or stable release.'
