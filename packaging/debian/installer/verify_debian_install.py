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
import shlex
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import zlib

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
udevadm settle || refuse "udev discovery did not settle"
set -- $(list-devices disk)
[ "$#" -eq 1 ] || refuse "expected exactly one scratch disk"
disk=$1
base=${disk##*/}
[ -b "$disk" ] || refuse "not a block device"
properties=$(udevadm info --query=property --name "$disk") || refuse "udev disk identity unavailable"
printf '%s\n' "$properties" | grep -Fx 'ID_SERIAL_SHORT=locasos-scratch' >/dev/null || refuse "wrong scratch serial"
printf '%s\n' "$properties" | grep -Fx 'ID_VENDOR=QEMU' >/dev/null || refuse "wrong scratch vendor"
printf '%s\n' "$properties" | grep -Fx 'ID_MODEL=QEMU_HARDDISK' >/dev/null || refuse "wrong scratch model"
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
echo "LOCASOS_CI_BLKID_STATUS $status" >/dev/ttyS0
# Debian's util-linux blkid returns 2 for a successfully probed blank disk.
# Do not accept an unknown implementation's status silently.
[ "$status" -eq 2 ] && [ -z "$signatures" ] || refuse "existing filesystem or unreadable signature probe"
while read -r source rest; do
    case "$source" in "$disk"|"$disk"[0-9]*|"$disk"p[0-9]*) refuse "mounted disk" ;; esac
done </proc/mounts
. /usr/share/debconf/confmodule
db_set partman-auto/disk "$disk"
'''

# No reusable login credential: the test user and root are locked. This disk is
# destroyed when the temporary directory exits; production prompts for an admin.
PRESEED = r'''d-i preseed/early_command string /usr/lib/locasos/guard-vm.sh
d-i partman/early_command string /usr/lib/locasos/ci-guard.sh
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


def validate_boot_menu(directory):
    """Check the shipped GRUB installer entries before bypassing GRUB in CI.

    Support live-build's fixed installer include, or an inline installer entry.
    This validates the distributed menu while the actual lifecycle test boots
    the same kernel/initrd directly to supply a test-only, unattended preseed.
    """
    directory = Path(directory)
    root = directory / "grub.cfg"
    if not root.is_file() or root.is_symlink():
        raise ValueError("ISO GRUB menu missing")
    configs = {}
    for path in directory.rglob("*.cfg"):
        if path.is_symlink() or not path.is_file():
            raise ValueError("ISO GRUB config must be a regular file")
        configs[path.relative_to(directory).as_posix()] = path.read_text(encoding="utf-8")
    source_line = re.compile(r"(?m)^\s*source\s+['\"]?/boot/grub/install_start\.cfg['\"]?\s*$")
    inline = re.compile(r"(?ims)^\s*menuentry\s+[^\n]*install[^\n]*\{(.*?)^\s*\}")
    # Commented examples must not count as a bootable installer entry.
    active_root = "\n".join(line for line in configs["grub.cfg"].splitlines()
                              if not line.lstrip().startswith("#"))
    sourced = bool(source_line.search(active_root))
    inline_entries = inline.findall(active_root)
    if not sourced and not inline_entries:
        raise ValueError("GRUB has no wired installer entry")
    if sourced:
        if "install_start.cfg" not in configs:
            raise ValueError("GRUB installer include missing")
        disabled = re.compile(r"(?is)\bif\s+false\s*;?\s*then\b(?:(?!\bfi\b).)*"
                              r"source\s+/boot/grub/install_start\.cfg")
        if disabled.search(active_root):
            raise ValueError("GRUB installer include is disabled")
    checked_kernel = checked_initrd = 0
    contents = [(name, body) for name, body in configs.items()
                if Path(name).name.startswith("install")]
    contents += [("inline installer", body) for body in inline_entries]
    # Any installer kernel elsewhere in grub.cfg must meet the same contract.
    contents += [("grub.cfg installer paths", "\n".join(
        line for line in active_root.splitlines() if "/install/" in line))]
    for name, body in contents:
        for line in body.splitlines():
            try:
                words = shlex.split(line, comments=True)
            except ValueError as error:
                raise ValueError("invalid GRUB installer syntax: " + name) from error
            if not words:
                continue
            if words[0] in ("linux", "linuxefi", "linux16"):
                if len(words) < 2 or words[1] != "/install/vmlinuz":
                    raise ValueError("GRUB installer uses an unexpected kernel: " + name)
                for argument in words[2:]:
                    if ("preseed" in argument or argument.startswith(("url=", "file="))
                            or "://" in argument):
                        raise ValueError("GRUB installer overrides embedded preseed: " + name)
                checked_kernel += 1
            elif words[0] in ("initrd", "initrdefi", "initrd16"):
                if words[1:] != ["/install/initrd.gz"]:
                    raise ValueError("GRUB installer uses an unexpected initrd: " + name)
                checked_initrd += 1
    if not checked_kernel or checked_kernel != checked_initrd:
        raise ValueError("GRUB installer needs matching kernel and initrd entries")
    if sourced:
        start = configs["install_start.cfg"]
        if not re.search(r"(?m)^\s*linux(?:efi)?\s+/install/vmlinuz(?:\s|$)", start):
            raise ValueError("GRUB start menu lacks installer kernel")
        if not re.search(r"(?m)^\s*initrd(?:efi)?\s+/install/initrd\.gz\s*$", start):
            raise ValueError("GRUB start menu lacks installer initrd")
    print("PASS: shipped GRUB installer menu uses the guarded kernel/initrd")


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


def initrd_members(source):
    """Read regular newc metadata without extracting untrusted initrd paths.

    Debian installer initrds are gzip-compressed newc archives. Linux permits
    concatenated compressed archives and zero padding; inspect the final member
    at each path because that is what the kernel exposes after unpacking.
    """
    try:
        raw = gzip.decompress(Path(source).read_bytes())
    except (OSError, EOFError, zlib.error) as error:
        raise ValueError("invalid gzip installer initrd") from error
    members = {}
    offset = 0
    while offset < len(raw):
        while offset < len(raw) and raw[offset] == 0:
            offset += 1
        if offset == len(raw):
            break
        if raw[offset:offset + 6] not in (b"070701", b"070702") or offset + 110 > len(raw):
            raise ValueError("invalid newc installer initrd header")
        magic = raw[offset:offset + 6]
        try:
            fields = [int(raw[offset + 6 + n * 8:offset + 14 + n * 8], 16)
                      for n in range(13)]
        except ValueError as error:
            raise ValueError("invalid newc numeric metadata") from error
        mode, size, namesize = fields[1], fields[6], fields[11]
        name_start = offset + 110
        name_end = name_start + namesize
        if namesize < 1 or name_end > len(raw) or raw[name_end - 1] != 0:
            raise ValueError("invalid newc member name")
        name = raw[name_start:name_end - 1].decode("utf-8", "surrogateescape")
        content_start = (name_end + 3) & ~3
        content_end = content_start + size
        if content_end > len(raw):
            raise ValueError("truncated newc member content")
        content = raw[content_start:content_end]
        if magic == b"070702" and (sum(content) & 0xffffffff) != fields[12]:
            raise ValueError("invalid newc member checksum")
        offset = (content_end + 3) & ~3
        if name == "TRAILER!!!":
            continue
        normalized = name.removeprefix("./").removeprefix("/")
        if ".." in Path(normalized).parts:
            raise ValueError("unsafe installer initrd member")
        members[normalized] = (mode, content)
    return members


def validate_production_initrd(source):
    """Require shipped interactive safety wiring before adding CI overrides."""
    members = initrd_members(source)
    for name in ("preseed.cfg", "usr/lib/locasos/guard-vm.sh",
                 "usr/lib/locasos/finish-install.sh"):
        if name not in members:
            raise ValueError("production installer initrd missing " + name)
        mode, content = members[name]
        if mode & 0o170000 != 0o100000 or not content:
            raise ValueError("production installer member must be a nonempty regular file: " + name)
        if name.endswith(".sh") and not mode & 0o111:
            raise ValueError("production installer hook is not executable: " + name)
    try:
        settings = [line.strip() for line in members["preseed.cfg"][1].decode().splitlines()
                    if line.strip() and not line.lstrip().startswith("#")]
    except UnicodeDecodeError as error:
        raise ValueError("production preseed is not UTF-8") from error
    for required in (
            "d-i preseed/early_command string /usr/lib/locasos/guard-vm.sh",
            "d-i preseed/late_command string /usr/lib/locasos/finish-install.sh"):
        matching = [line for line in settings if line.split()[:2] == required.split()[:2]]
        if matching != [required]:
            raise ValueError("production installer safety command missing or overridden: " + required)
    if any("partman" in line or "passwd/user-password" in line
           or any(key in line for key in ("preseed/include", "preseed/run"))
           for line in settings):
        raise ValueError("production preseed must not automate disk partitioning or credentials")
    print("PASS: shipped installer initrd has interactive VM guard and finish hooks")


def prepare_initrd(source, destination):
    validate_production_initrd(source)
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
                        "-extract", "/install/initrd.gz", str(initrd),
                        "-extract", "/boot/grub", str(tmp / "grub")], check=True)
        validate_boot_menu(tmp / "grub")
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
