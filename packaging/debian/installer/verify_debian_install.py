#!/usr/bin/env python3
"""Install the exact developer ISO into a new disposable QEMU disk, then boot twice.

The production ISO is never modified. Unattended partitioning is present ONLY
in a temporary installer initrd guarded by QEMU fw_cfg and synthetic disk ID.
No option accepts a target disk, host block device, shared directory, or network.
"""
import argparse
import gzip
import os
from pathlib import Path
import re
import selectors
import shutil
import socket
import subprocess
import sys
import tempfile
import time

from disposable_vm import checksum
from guest_serial import StageEvidence

TOKEN = "LOCASOS_DISPOSABLE_VM_INSTALL_V1"
TOOLS = ("xorriso", "qemu-img", "qemu-system-x86_64")
CI_GUARD = r'''#!/bin/sh
set -eu
refuse() {
    echo "LOCASOS_VM_INSTALL_REFUSED: $*" >/dev/ttyS0
    poweroff -f || true
    # Never return into a preseed containing partitioning confirmations.
    while :; do sleep 60; done
}
/usr/lib/locasos/guard-vm.sh || refuse "production VM guard"
modprobe qemu_fw_cfg || refuse "qemu_fw_cfg unavailable"
marker=/sys/firmware/qemu_fw_cfg/by_name/opt/locasos/dev-install/raw
[ -r "$marker" ] || refuse "QEMU opt-in marker missing"
[ "$(cat "$marker")" = LOCASOS_DISPOSABLE_VM_INSTALL_V1 ] || refuse "wrong QEMU opt-in marker"
set -- $(list-devices disk)
[ "$#" -eq 1 ] || refuse "expected exactly one scratch disk"
disk=$1
base=${disk##*/}
[ -b "$disk" ] || refuse "not a block device"
[ "$(cat "/sys/class/block/$base/device/serial")" = locasos-scratch ] || refuse "wrong scratch serial"
[ "$(cat "/sys/class/block/$base/ro")" = 0 ] || refuse "read-only disk"
sectors=$(cat "/sys/class/block/$base/size")
case "$sectors" in ''|*[!0-9]*) refuse "unknown disk size" ;; esac
[ "$sectors" -ge 134217728 ] || refuse "disk smaller than 64 GiB"
for child in /sys/class/block/"$base"/"$base"*; do
    [ ! -e "$child/partition" ] || refuse "disk contains partitions"
done
# Existing filesystem or partition signatures must never be accepted.
command -v blkid >/dev/null || refuse "blkid unavailable"
set +e
signatures=$(blkid "$disk" 2>/dev/null)
status=$?
set -e
[ "$status" -eq 2 ] && [ -z "$signatures" ] || refuse "existing filesystem or unreadable signature probe"
while read -r source rest; do
    case "$source" in "$disk"|"$disk"[0-9]*|"$disk"p[0-9]*) refuse "mounted disk" ;; esac
done </proc/mounts
. /usr/share/debconf/confmodule
db_set partman-auto/disk "$disk"
'''

# No reusable login credential: the test user and root are locked. This disk is
# destroyed when the temporary directory exits; production prompts for an admin.
PRESEED = r'''d-i preseed/early_command string /usr/lib/locasos/ci-guard.sh
# Locale and network are deterministic and no package mirror is reachable.
d-i debian-installer/locale string en_US.UTF-8
d-i keyboard-configuration/xkb-keymap select us
d-i netcfg/enable boolean false
d-i netcfg/get_hostname string locasos-vm-test
d-i netcfg/get_domain string local
d-i hw-detect/load_firmware boolean false
d-i anna/choose_modules string live-installer
d-i live-installer/enable boolean true
d-i live-installer/mode select normal
d-i apt-setup/use_mirror boolean false
d-i apt-setup/services-select multiselect
d-i apt-setup/cdrom/set-first boolean false
d-i clock-setup/utc boolean true
d-i time/zone string Etc/UTC
d-i clock-setup/ntp boolean false
d-i passwd/root-login boolean false
d-i passwd/user-fullname string Disposable CI operator
d-i passwd/username string vmtest
d-i passwd/user-password-crypted password !
d-i user-setup/allow-password-weak boolean false
d-i partman-auto/method string regular
d-i partman-auto/choose_recipe select atomic
d-i partman-partitioning/choose_label select gpt
d-i partman-partitioning/default_label string gpt
d-i partman-partitioning/confirm_write_new_label boolean true
d-i partman/choose_partition select finish
d-i partman/confirm boolean true
d-i partman/confirm_nooverwrite boolean true
d-i partman-efi/non_efi_system boolean false
d-i grub-installer/only_debian boolean true
d-i grub-installer/with_other_os boolean false
d-i grub-installer/force-efi-extra-removable boolean true
tasksel tasksel/first multiselect
d-i pkgsel/upgrade select none
d-i popularity-contest/participate boolean false
d-i preseed/late_command string /usr/lib/locasos/ci-finish.sh
d-i finish-install/reboot_in_progress note
d-i debian-installer/exit/poweroff boolean true
'''

CI_FINISH = r'''#!/bin/sh
set -eu
/usr/lib/locasos/finish-install.sh
mkdir -p /target/var/lib/locas
printf 'locasos-vm-install-v1\n' >/target/var/lib/locas/.locasos-vm-install-sentinel
touch /target/etc/locasos-installed-vm-test
cat >>/target/etc/default/grub <<'GRUB'
GRUB_CMDLINE_LINUX="console=tty0 console=ttyS0,115200n8"
GRUB_TERMINAL="console serial"
GRUB_SERIAL_COMMAND="serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1"
GRUB
in-target update-grub
sync
# The host also waits for installer poweroff before starting disk-only boots.
printf 'LOCASOS_VM_INSTALL_COMPLETE\n' >/dev/ttyS0
'''


def checked_file(path):
    """Reject block devices and QEMU option separators in user-supplied paths."""
    path = Path(path).resolve(strict=True)
    if not path.is_file() or any(c in str(path) for c in (",", "\n", "\r")):
        raise ValueError("expected a regular file with a QEMU-safe path: " + str(path))
    return path


def validate_inputs(iso, digest, code, variables):
    for tool in TOOLS:
        if not shutil.which(tool):
            raise ValueError(tool + " missing")
    iso, code, variables = map(checked_file, (iso, code, variables))
    if not re.fullmatch(r"[0-9a-fA-F]{64}", digest) or checksum(iso) != digest.lower():
        raise ValueError("ISO digest mismatch")
    return iso, code, variables


def newc_archive(files):
    """Small fixed-input newc archive, appended as a second compressed initrd."""
    data = bytearray()
    for inode, (name, content, mode) in enumerate([*files, ("TRAILER!!!", b"", 0)], 1):
        if name.startswith("/") or ".." in Path(name).parts:
            raise ValueError("unsafe initrd member")
        encoded = name.encode() + b"\0"
        fields = (inode, mode, 0, 0, 1, 0, len(content), 0, 0, 0, 0, len(encoded), 0)
        data.extend(b"070701" + b"".join(f"{field:08x}".encode() for field in fields))
        data.extend(encoded)
        data.extend(b"\0" * (-len(data) % 4))
        data.extend(content)
        data.extend(b"\0" * (-len(data) % 4))
    return bytes(data)


def prepare_initrd(source, destination):
    files = [("preseed.cfg", PRESEED.encode(), 0o100644),
             ("usr/lib/locasos/ci-guard.sh", CI_GUARD.encode(), 0o100755),
             ("usr/lib/locasos/ci-finish.sh", CI_FINISH.encode(), 0o100755)]
    with Path(destination).open("wb") as out:
        with Path(source).open("rb") as original:
            shutil.copyfileobj(original, out)
        out.write(gzip.compress(newc_archive(files), mtime=0))


def vm_command(code, variables, disk, monitor, iso=None, kernel=None, initrd=None):
    cmd = ["qemu-system-x86_64", "-nodefaults", "-machine", "q35,accel=tcg",
           "-m", "4096", "-smp", "2", "-display", "none", "-serial", "stdio",
           "-monitor", "unix:" + str(monitor) + ",server=on,wait=off",
           "-no-reboot", "-net", "none",
           "-drive", "if=pflash,format=raw,readonly=on,file=" + str(code),
           "-drive", "if=pflash,format=raw,file=" + str(variables),
           "-device", "virtio-scsi-pci",
           "-drive", "if=none,id=scratch,format=qcow2,file=" + str(disk),
           "-device", "scsi-hd,drive=scratch,serial=locasos-scratch"]
    if iso is not None:
        if kernel is None or initrd is None:
            raise ValueError("installer kernel and temporary initrd required")
        cmd += ["-drive", "if=none,id=cd,format=raw,readonly=on,media=cdrom,file=" + str(iso),
                "-device", "scsi-cd,drive=cd", "-kernel", str(kernel), "-initrd", str(initrd),
                "-append", "console=ttyS0,115200n8 auto=true priority=critical preseed/file=/preseed.cfg",
                "-fw_cfg", "name=opt/locasos/dev-install,string=" + TOKEN]
    elif kernel is not None or initrd is not None:
        raise ValueError("disk-only boots must not supply kernel or initrd")
    else:
        cmd += ["-boot", "order=c"]
    return cmd


def run_stage(command, stage, timeout, monitor, logfile):
    """Require fresh guest evidence and clean poweroff; never accept mere uptime."""
    evidence = StageEvidence(stage)
    logs = bytearray()
    proc = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
    powered_down = False
    accepted_deadline = None
    try:
        with selectors.DefaultSelector() as events:
            events.register(proc.stdout, selectors.EVENT_READ)
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                for key, _ in events.select(timeout=1):
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        events.unregister(key.fileobj)
                        continue
                    logs.extend(chunk)
                    if len(logs) > 4 * 1024 * 1024:
                        del logs[:1024 * 1024]
                    evidence.feed(chunk)
                if evidence.accepted and accepted_deadline is None:
                    accepted_deadline = time.monotonic() + 120
                if evidence.accepted and stage == "installed" and not powered_down:
                    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as control:
                        control.settimeout(3)
                        control.connect(str(monitor))
                        control.sendall(b"system_powerdown\n")
                    powered_down = True
                if proc.poll() is not None:
                    if not evidence.accepted or proc.returncode != 0:
                        raise RuntimeError(f"{stage}: guest exited without successful evidence (exit {proc.returncode})")
                    return
                if accepted_deadline is not None and time.monotonic() > accepted_deadline:
                    raise RuntimeError(stage + ": guest did not power off after successful evidence")
            raise RuntimeError(stage + ": guest evidence timed out")
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
        if proc.stdout is not None:
            proc.stdout.close()
        Path(logfile).write_bytes(logs)
        print(logs[-12000:].decode("utf-8", "replace"), flush=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iso", type=Path, required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--ovmf-code", type=Path, required=True)
    parser.add_argument("--ovmf-vars", type=Path, required=True)
    parser.add_argument("--install-timeout", type=int, default=1200)
    parser.add_argument("--boot-timeout", type=int, default=360)
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args(argv)
    if not (60 <= args.install_timeout <= 3600 and 60 <= args.boot_timeout <= 900):
        parser.error("timeouts outside bounded acceptance range")
    iso, code, variables = validate_inputs(args.iso, args.sha256, args.ovmf_code, args.ovmf_vars)
    args.log_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="locasos-debian-install-") as tmp:
        tmp = Path(tmp)
        kernel, initrd = tmp / "vmlinuz", tmp / "original-initrd.gz"
        subprocess.run(["xorriso", "-osirrox", "on", "-indev", str(iso),
                        "-extract", "/install/vmlinuz", str(kernel),
                        "-extract", "/install/initrd.gz", str(initrd)], check=True)
        test_initrd = tmp / "ci-initrd.gz"
        prepare_initrd(initrd, test_initrd)
        disk, nvram, monitor = tmp / "scratch.qcow2", tmp / "vars.fd", tmp / "monitor.sock"
        shutil.copyfile(variables, nvram)
        subprocess.run(["qemu-img", "create", "-f", "qcow2", str(disk), "64G"], check=True)
        run_stage(vm_command(code, nvram, disk, monitor, iso, kernel, test_initrd),
                  "install", args.install_timeout, monitor, args.log_dir / "install.serial.log")
        # Reuse only this newly-created disk and its guest NVRAM. Neither boot
        # has the ISO, injected initrd, kernel, fw_cfg install token, or network.
        for number in (1, 2):
            monitor.unlink(missing_ok=True)
            run_stage(vm_command(code, nvram, disk, monitor), "installed", args.boot_timeout,
                      monitor, args.log_dir / f"disk-boot-{number}.serial.log")
    print("PASS: Debian live-image installation and two disk-only UEFI boots with persistent health evidence")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as error:
        sys.exit("FAIL: " + str(error))
