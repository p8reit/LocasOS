#!/usr/bin/env python3
"""Developer-only guest boot readiness and bounded failure diagnostics."""
import argparse
import json
import pathlib
import subprocess
import sys
import time
import urllib.request

HEALTH_URL = "http://127.0.0.1:59420/healthz"
SERVICES = ("locas-agent.service", "locas-core.service")
MARKER = "LOCASOS_GUEST_HEALTH_OK"
FAIL_MARKER = "LOCASOS_GUEST_HEALTH_FAILED"


def healthy_payload(raw):
    """Do not accept HTTP 200 alone, or a nested/unrelated status."""
    try:
        data = json.loads(raw)
    except (TypeError, ValueError):
        return False
    return (type(data) is dict and data.get("component") == "locas-core"
            and data.get("status") == "ok")


def systemd_ready():
    try:
        for unit in ("multi-user.target", *SERVICES):
            state = subprocess.run(
                ["systemctl", "is-active", "--quiet", unit],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                timeout=3, check=False,
            )
            if state.returncode != 0:
                return False
        return True
    except (OSError, subprocess.TimeoutExpired):
        return False


def api_ready():
    try:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(HEALTH_URL, timeout=3) as response:
            return response.status == 200 and healthy_payload(response.read(8192))
    except (OSError, ValueError, TimeoutError):
        return False


def diag(serial):
    """Serial diagnostics only in developer guest; never dump environment/secrets."""
    for unit in ("dbus.service", "locas-firstboot.service",
                 "locas-agent.service", "locas-core.service"):
        serial.write(f"LOCASOS_DIAG_UNIT {unit}\n")
        try:
            output = subprocess.run(
                ["systemctl", "show", unit, "--property=ActiveState,SubState,Result",
                 "--no-pager"], capture_output=True, text=True,
                timeout=5, check=False,
            ).stdout
            serial.write(output[:2048])
        except (OSError, subprocess.TimeoutExpired):
            serial.write("status unavailable\n")
        try:
            # Last thirty journal lines, capped; do not log process environment.
            output = subprocess.run(
                ["journalctl", "-b", "-u", unit, "-n", "30",
                 "--no-pager", "--output=short"],
                capture_output=True, text=True, timeout=5, check=False,
            ).stdout
            serial.write(output[-4096:])
        except (OSError, subprocess.TimeoutExpired):
            serial.write("journal unavailable\n")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--serial", type=pathlib.Path, default=pathlib.Path("/dev/ttyS0"))
    p.add_argument("--timeout", type=int, default=240)
    args = p.parse_args()
    if not 1 <= args.timeout <= 300:
        p.error("invalid bounded timeout")
    if not pathlib.Path("/etc/locasos-release").is_file():
        print("not a LocasOS guest", file=sys.stderr)
        return 2
    if 'EDITION="native-developer-preview"' not in pathlib.Path(
            "/etc/locasos-release").read_text(encoding="utf-8"):
        print("wrong LocasOS edition", file=sys.stderr)
        return 2
    with args.serial.open("w", buffering=1) as serial:
        deadline = time.monotonic() + args.timeout
        while time.monotonic() < deadline:
            if systemd_ready() and api_ready():
                serial.write(MARKER + "\n")
                if pathlib.Path("/etc/locasos-installed-vm-test").is_file():
                    sentinel = pathlib.Path("/var/lib/locas/.locasos-vm-install-sentinel")
                    try:
                        persisted = sentinel.read_text(encoding="utf-8").strip() == "locasos-vm-install-v1"
                    except OSError:
                        persisted = False
                    if persisted:
                        serial.write("LOCASOS_INSTALLED_BOOT_OK\n")
                    else:
                        serial.write("LOCASOS_INSTALLED_DATA_FAILED\n")
                        return 1
                return 0
            time.sleep(2)
        serial.write(FAIL_MARKER + "\n")
        diag(serial)
    return 1


if __name__ == "__main__":
    sys.exit(main())
