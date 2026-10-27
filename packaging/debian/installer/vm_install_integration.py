#!/usr/bin/env python3
"""Two-boot integration test: disposable live ISO -> scratch disk -> installed boot."""
import argparse
import os
from pathlib import Path
import select
import shutil
import subprocess
import sys
import tempfile
import time
from disposable_vm import checksum

MARKER = "locasos-ephemeral-disk-v1"

def ovmf_paths():
    for base in ("/usr/share/OVMF", "/usr/share/ovmf"):
        code = Path(base) / "OVMF_CODE_4M.fd"
        var = Path(base) / "OVMF_VARS_4M.fd"
        if code.is_file() and var.is_file():
            return code, var
        code = Path(base) / "OVMF_CODE.fd"
        var = Path(base) / "OVMF_VARS.fd"
        if code.is_file() and var.is_file():
            return code, var
    raise RuntimeError("Matching OVMF CODE and VARS firmware not installed")

def qemu_args(iso, disk, code, var, from_iso):
    args = [
        "qemu-system-x86_64", "-machine", "q35,accel=tcg", "-cpu", "max",
        "-m", "3072", "-smp", "2", "-display", "none", "-serial", "stdio",
        "-monitor", "none", "-net", "none", "-no-reboot",
        "-smbios", "type=1,manufacturer=QEMU,product=LocasOS-QEMU-InstallTest",
        "-drive", "if=pflash,format=raw,readonly=on,file=" + str(code),
        "-drive", "if=pflash,format=raw,file=" + str(var),
        "-device", "virtio-blk-pci,drive=scratch",
        "-drive", "if=none,id=scratch,file=" + str(disk) + ",format=qcow2,cache=none",
        "-boot", "order=" + ("d" if from_iso else "c"),
    ]
    if from_iso:
        args.extend([
            "-fw_cfg", "name=opt/locasos/ephemeral,string=" + MARKER,
            "-drive", "file=" + str(iso) + ",media=cdrom,if=none,id=cd,readonly=on,format=raw",
            "-device", "ich9-ahci,id=ahci",
            "-device", "ide-cd,bus=ahci.0,drive=cd",
        ])
    return args

def await_markers(command, markers, deadline_seconds, log_file):
    print("Starting isolated QEMU test phase; waiting for " + ", ".join(markers), flush=True)
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               bufsize=0)
    pending = bytearray()
    observed = set()
    deadline = time.monotonic() + deadline_seconds
    try:
        with open(log_file, "ab") as log:
            while time.monotonic() < deadline:
                ready, _, _ = select.select([process.stdout], [], [], 1)
                if ready:
                    block = os.read(process.stdout.fileno(), 8192)
                    if block:
                        log.write(block)
                        log.flush()
                        pending.extend(block)
                        while b"\n" in pending:
                            line, _, remainder = pending.partition(b"\n")
                            pending = bytearray(remainder)
                            text = line.decode(errors="replace").strip()
                            print(text[-220:], flush=True)
                            for mark in markers:
                                if mark in text:
                                    observed.add(mark)
                        if len(pending) > 32768:
                            pending = pending[-4096:]
                    elif process.poll() is not None:
                        break
                elif process.poll() is not None:
                    break
                if observed.issuperset(markers):
                    return True
            return False
    finally:
        process.terminate()
        try:
            process.wait(timeout=8)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iso", required=True, type=Path)
    parser.add_argument("--sha256")
    parser.add_argument("--live-timeout", type=int, default=480)
    parser.add_argument("--installed-timeout", type=int, default=240)
    parser.add_argument("--log-dir", type=Path)
    args = parser.parse_args()
    iso = args.iso.resolve(strict=True)
    if args.sha256 and checksum(iso) != args.sha256.lower():
        parser.error("ISO checksum mismatch")
    for tool in ("qemu-system-x86_64", "qemu-img"):
        if not shutil.which(tool):
            parser.error("Missing " + tool)
    code, var_template = ovmf_paths()
    with tempfile.TemporaryDirectory(prefix="locasos-vm-install-") as directory:
        workspace = Path(directory)
        disk = workspace / "only-disk.qcow2"
        vars_file = workspace / "OVMF_VARS.fd"
        shutil.copy2(var_template, vars_file)
        subprocess.run(["qemu-img", "create", "-f", "qcow2", str(disk), "64G"], check=True)
        logs = args.log_dir.resolve() if args.log_dir else workspace
        logs.mkdir(parents=True, exist_ok=True)
        live_log = logs / "live-boot.log"
        installed_log = logs / "installed-boot.log"
        if not await_markers(qemu_args(iso,disk,code,vars_file,True),
                             {"LOCASOS_LIVE_READY", "LOCASOS_VM_INSTALL_COMPLETE"},
                             args.live_timeout,live_log):
            raise RuntimeError("Live boot or full scratch-disk install not confirmed; inspect " + str(live_log))
        # Test next cold boot from the scratch disk only; live ISO and install marker are removed.
        if not await_markers(qemu_args(iso,disk,code,vars_file,False),
                             {"LOCASOS_INSTALLED_READY"}, args.installed_timeout,installed_log):
            raise RuntimeError("Installed disk boot and healthy Core not confirmed; inspect " + str(installed_log))
        print("PASS: live Core healthy, scratch disk installed, installed Core healthy on UEFI disk boot")
    return 0

if __name__=="__main__":
    try: sys.exit(main())
    except (OSError, RuntimeError, subprocess.CalledProcessError) as exc:
        print("FAIL: " + str(exc),file=sys.stderr)
        sys.exit(1)
