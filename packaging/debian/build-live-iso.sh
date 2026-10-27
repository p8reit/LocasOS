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
mkdir -p "$workspace/config/package-lists" "$workspace/config/includes.chroot/usr/local/share/locasos" "$workspace/config/hooks/live"
# Source-build payload remains explicitly VM-only and is NOT executed by the live image.
install -m 0644 dist/native/locasos-native-payload-amd64.tar "$workspace/config/includes.chroot/usr/local/share/locasos/"
install -m 0644 dist/native/locasos-native-payload-amd64.tar.sha256 "$workspace/config/includes.chroot/usr/local/share/locasos/"
# VM-only developer readiness marker and disposable QEMU install machinery.
mkdir -p "$workspace/config/includes.chroot/usr/local/libexec" "$workspace/config/includes.chroot/etc/systemd/system"
install -m 0755 packaging/debian/installer/guest-health.sh "$workspace/config/includes.chroot/usr/local/libexec/locasos-guest-health.sh"
install -m 0755 packaging/debian/installer/install-qemu-only.sh "$workspace/config/includes.chroot/usr/local/libexec/locasos-install-qemu-only.sh"
install -m 0644 packaging/debian/installer/locasos-guest-health.service "$workspace/config/includes.chroot/etc/systemd/system/"
install -m 0644 packaging/debian/installer/locasos-install-qemu-only.service "$workspace/config/includes.chroot/etc/systemd/system/"
# Reuse exactly the same baseline system dependencies as WSL.
grep -Ev '^[[:space:]]*(#|$)' packaging/common/base-packages.txt > "$workspace/config/package-lists/locasos.list.chroot"
cat >> "$workspace/config/package-lists/locasos.list.chroot" <<'LIST'
openssh-client
pciutils
python3
curl
rsync
gdisk
parted
dosfstools
e2fsprogs
grub-efi-amd64-bin
grub2-common
initramfs-tools
LIST
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
systemctl enable locas-firstboot.service locas-agent.service locas-core.service
systemctl enable locasos-guest-health.service locasos-install-qemu-only.service
# Path workers are not enabled by default until native lifecycle approval is complete.
HOOK
chmod 0755 "$workspace/config/hooks/live/"*.hook.chroot
# Headless appliances must boot the default live entry without a keyboard.
# Preserve Debian live-build's GRUB template; override only the menu timeout.
grub_template="/usr/share/live/build/bootloaders/grub-pc/config.cfg"
[[ -f "$grub_template" ]] || { echo "Missing live-build GRUB template: $grub_template" >&2; exit 1; }
mkdir -p "$workspace/config/bootloaders/grub-pc"
cp "$grub_template" "$workspace/config/bootloaders/grub-pc/config.cfg"
printf '\nset timeout=3\n' >> "$workspace/config/bootloaders/grub-pc/config.cfg"
cd "$workspace"
lb config --mode debian --distribution trixie --architectures amd64 \
  --binary-images iso-hybrid --linux-flavours amd64 --linux-packages linux-image \
  --debian-installer none --archive-areas "main" \
  --bootappend-live "boot=live components console=ttyS0,115200" \
  --iso-application "LocasOS Developer Live" --iso-publisher "LocasOS Community" \
  --iso-volume "LOCASOS_DEV" --apt-recommends false
lb build
mkdir -p "$repo/dist/debian"
image="$(find . -maxdepth 1 -type f -name '*.iso' -print -quit)"
[[ -n "$image" ]] || { echo 'Live-build did not produce an ISO' >&2; exit 1; }
install -m 0644 "$image" "$repo/dist/debian/locasos-native-developer-amd64.iso"
(cd "$repo/dist/debian" && sha256sum locasos-native-developer-amd64.iso > locasos-native-developer-amd64.iso.sha256)
echo 'Built Debian-based LocasOS developer live ISO; NOT an OS installer or stable release.'
