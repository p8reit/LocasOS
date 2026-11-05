#!/bin/sh
# Runs only as Debian Installer's late_command, after interactive disk consent.
set -eu
fail() {
    echo "LOCASOS_VM_INSTALL_FAIL: $*" >&2
    exit 1
}
[ "$(id -u)" = 0 ] || fail 'installer root required'
[ -d /var/lib/cdebconf ] && command -v in-target >/dev/null || fail 'Debian Installer required'
/usr/lib/locasos/guard-vm.sh
grep -q ' /target ' /proc/mounts || fail 'no installed target mounted'
[ ! -L /target ] || fail 'symlink target refused'
grep -qx 'EDITION="native-developer-preview"' /target/etc/locasos-release || fail 'wrong target edition'
[ -x /target/opt/locas/bin/locas-core ] || fail 'LocasOS payload missing'
# A real account must have been configured by Debian's interactive user setup.
awk -F: '$3 >= 1000 && $3 < 65534 && $7 !~ /(nologin|false)$/ { found=1 } END { exit !found }' /target/etc/passwd || fail 'operator account missing'
# Do not clone build or live-session identities into the installed VM.
rm -f /target/etc/machine-id /target/var/lib/dbus/machine-id
: > /target/etc/machine-id
ln -s /etc/machine-id /target/var/lib/dbus/machine-id
rm -f /target/etc/ssh/ssh_host_*
in-target ssh-keygen -A
in-target systemctl enable ssh.service locas-firstboot.service locas-agent.service locas-core.service locasos-guest-readiness.service
# This marker also distinguishes real disk boots from successful live boots.
touch /target/etc/locasos-installed-vm
mkdir -p /target/var/lib/locas
chmod 0750 /target/var/lib/locas
printf 'locasos-vm-install-v1\n' > /target/var/lib/locas/.locasos-vm-install-sentinel
in-target update-initramfs -u -k all
sync
printf 'LOCASOS_VM_INSTALL_COMPLETE\n'
