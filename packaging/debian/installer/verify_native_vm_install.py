#!/usr/bin/env python3
"""End-to-end disposable QEMU install + reboot assertion; scratch qcow2 only."""
import argparse
import pathlib
import shutil
import subprocess
import sys
import tempfile
from disposable_vm import checksum
from guest_serial import StageEvidence, SerialFailure

def vm_cmd(code, varsfile, disk, iso=None, enable_install=True, monitor_path=None):
    args = ["qemu-system-x86_64", "-nodefaults", "-machine", "q35,accel=tcg",
            "-m", "4096", "-smp", "2", "-vga", "std", "-display", "none", "-serial", "stdio",
            "-monitor", "none", "-no-reboot", "-net", "none",
            "-device", "qemu-xhci", "-device", "usb-kbd",
            "-drive", "if=pflash,format=raw,readonly=on,file=" + str(code),
            "-drive", "if=pflash,format=raw,file=" + str(varsfile),
            "-device", "virtio-scsi-pci",
            "-drive", "if=none,id=scratch,format=qcow2,file=" + str(disk),
            "-device", "scsi-hd,drive=scratch,serial=locasos-scratch"]
    if monitor_path is not None:
        args += ["-monitor", "unix:" + str(monitor_path) + ",server,nowait"]
        # Remove the inert monitor=none option, leaving the dedicated private socket.
        index = args.index("-monitor")
        del args[index:index + 2]
    if iso is not None:
        args += ["-drive", "if=none,id=cd,format=raw,readonly=on,media=cdrom,file=" + str(iso),
                 "-device", "scsi-cd,drive=cd",
                 "-boot", "order=d"]
        if enable_install:
            args += ["-fw_cfg", "name=opt/locasos/dev-install,string=LOCASOS_DISPOSABLE_VM_INSTALL_V1"]
    else:
        args += ["-boot", "order=c"]
    return args

def run_until_marker(cmd, stage, timeout, monitor_path=None, log_path=None):
    """Require fresh, exact guest serial evidence from this one VM process."""
    import os
    import selectors
    import socket
    import time
    evidence = StageEvidence(stage)
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
    logs = bytearray()
    try:
        with selectors.DefaultSelector() as selector:
            selector.register(proc.stdout, selectors.EVENT_READ)
            deadline = time.monotonic() + timeout
            next_enter = time.monotonic() + 15
            keypresses = 0
            while time.monotonic() < deadline:
                events = selector.select(timeout=min(1.0, max(0, deadline - time.monotonic())))
                if monitor_path is not None and keypresses < 8 and time.monotonic() >= next_enter:
                    try:
                        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as control:
                            control.settimeout(2)
                            control.connect(str(monitor_path))
                            control.sendall(b"sendkey ret\n")
                        keypresses += 1
                        next_enter = time.monotonic() + 20
                    except OSError:
                        next_enter = time.monotonic() + 3
                for key, _ in events:
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                        continue
                    logs.extend(chunk)
                    if len(logs) > 4 * 1024 * 1024:
                        del logs[:1024 * 1024]
                    evidence.feed(chunk)
                if evidence.accepted or proc.poll() is not None:
                    break
        if not evidence.accepted:
            raise RuntimeError("Guest marker not received for stage: " + stage)
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
        if log_path is not None:
            pathlib.Path(log_path).write_bytes(logs)
        print(logs[-12000:].decode("utf-8", "replace"), flush=True)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--iso", type=pathlib.Path, required=True)
    ap.add_argument("--sha256", required=True)
    ap.add_argument("--ovmf-code", type=pathlib.Path, required=True)
    ap.add_argument("--ovmf-vars", type=pathlib.Path, required=True)
    ap.add_argument("--live-timeout", type=int, default=480)
    ap.add_argument("--install-timeout", type=int, default=900)
    ap.add_argument("--boot-timeout", type=int, default=300)
    ap.add_argument("--log-dir", type=pathlib.Path)
    args = ap.parse_args()
    for tool in ("qemu-img", "qemu-system-x86_64"):
        if not shutil.which(tool): ap.error(tool + " missing")
    iso = args.iso.resolve(strict=True)
    if checksum(iso) != args.sha256.lower(): ap.error("ISO digest mismatch")
    with tempfile.TemporaryDirectory(prefix="locasos-e2e-qemu-") as tmp:
        disk = pathlib.Path(tmp) / "scratch.qcow2"
        varsfile = pathlib.Path(tmp) / "OVMF_VARS.fd"
        monitor = pathlib.Path(tmp) / "monitor.sock"
        shutil.copyfile(args.ovmf_vars, varsfile)
        subprocess.run(["qemu-img", "create", "-f", "qcow2", str(disk), "64G"], check=True)
        if args.log_dir:
            args.log_dir.mkdir(parents=True, exist_ok=True)
        def stage(iso_image, name, seconds, enable_install=False):
            monitor.unlink(missing_ok=True)
            command = vm_cmd(args.ovmf_code, varsfile, disk, iso_image, enable_install=enable_install, monitor_path=monitor)
            log = args.log_dir / (name + ".serial.log") if args.log_dir else None
            run_until_marker(command, name, seconds, monitor_path=monitor, log_path=log)
        stage(iso, "live", args.live_timeout)
        stage(iso, "install", args.install_timeout, enable_install=True)
        stage(None, "installed", args.boot_timeout)
    print("PASS: Exact serial evidence for QEMU live, scratch install and disk-only UEFI boot")

if __name__ == "__main__":
    try: main()
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as err:
        print("FAIL: " + str(err), file=sys.stderr)
        sys.exit(1)
