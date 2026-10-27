#!/usr/bin/env python3
"""End-to-end disposable QEMU install + reboot assertion; scratch qcow2 only."""
import argparse
import pathlib
import shutil
import subprocess
import sys
import tempfile
from disposable_vm import checksum

def vm_cmd(code, varsfile, disk, iso=None):
    args = ["qemu-system-x86_64", "-nodefaults", "-machine", "q35,accel=tcg",
            "-m", "4096", "-smp", "2", "-display", "none", "-serial", "stdio",
            "-monitor", "none", "-no-reboot", "-net", "none",
            "-drive", "if=pflash,format=raw,readonly=on,file=" + str(code),
            "-drive", "if=pflash,format=raw,file=" + str(varsfile),
            "-device", "virtio-scsi-pci",
            "-drive", "if=none,id=scratch,format=qcow2,file=" + str(disk),
            "-device", "scsi-hd,drive=scratch,serial=locasos-scratch"]
    if iso is not None:
        args += ["-drive", "if=none,id=cd,format=raw,readonly=on,media=cdrom,file=" + str(iso),
                 "-device", "scsi-cd,drive=cd",
                 "-fw_cfg", "name=opt/locasos/dev-install,string=LOCASOS_DISPOSABLE_VM_INSTALL_V1",
                 "-boot", "order=d"]
    else:
        args += ["-boot", "order=c"]
    return args

def run_until_marker(cmd, marker, timeout):
    try:
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout, errors="replace")
        output = p.stdout + "\n" + p.stderr
    except subprocess.TimeoutExpired as exc:
        output = (exc.stdout or b"").decode("utf-8", "replace") if isinstance(exc.stdout, bytes) else (exc.stdout or "")
        output += "\n" + ((exc.stderr or b"").decode("utf-8", "replace") if isinstance(exc.stderr, bytes) else (exc.stderr or ""))
    print(output[-12000:], flush=True)
    if "LOCASOS_VM_INSTALL_FAIL" in output or "LOCASOS_GUEST_HEALTH_FAILED" in output:
        raise RuntimeError("Guest reported an explicit failure")
    if marker not in output:
        raise RuntimeError("Expected guest serial marker missing: " + marker)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--iso", type=pathlib.Path, required=True)
    ap.add_argument("--sha256", required=True)
    ap.add_argument("--ovmf-code", type=pathlib.Path, required=True)
    ap.add_argument("--ovmf-vars", type=pathlib.Path, required=True)
    ap.add_argument("--install-timeout", type=int, default=900)
    ap.add_argument("--boot-timeout", type=int, default=300)
    args = ap.parse_args()
    for tool in ("qemu-img", "qemu-system-x86_64"):
        if not shutil.which(tool): ap.error(tool + " missing")
    iso = args.iso.resolve(strict=True)
    if checksum(iso) != args.sha256.lower(): ap.error("ISO digest mismatch")
    with tempfile.TemporaryDirectory(prefix="locasos-e2e-qemu-") as tmp:
        disk = pathlib.Path(tmp) / "scratch.qcow2"
        varsfile = pathlib.Path(tmp) / "OVMF_VARS.fd"
        shutil.copyfile(args.ovmf_vars, varsfile)
        subprocess.run(["qemu-img", "create", "-f", "qcow2", str(disk), "64G"], check=True)
        run_until_marker(vm_cmd(args.ovmf_code, varsfile, disk, iso), "LOCASOS_VM_INSTALL_COMPLETE", args.install_timeout)
        run_until_marker(vm_cmd(args.ovmf_code, varsfile, disk), "LOCASOS_INSTALLED_BOOT_OK", args.boot_timeout)
    print("PASS: Scratch-only Debian LocasOS install, EFI reboot and Core health")

if __name__ == "__main__":
    try: main()
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as err:
        print("FAIL: " + str(err), file=sys.stderr)
        sys.exit(1)
