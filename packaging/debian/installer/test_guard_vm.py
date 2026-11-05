"""Exercise the read-only installer VM gate and its nonreturning refusal."""
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest


GUARD = Path(__file__).with_name("guard-vm.sh")


class VMGuardTests(unittest.TestCase):
    def fixture(self, directory, vendor="QEMU",
                product="Standard PC (Q35 + ICH9, 2009)", efi=True):
        root = Path(directory)
        dmi = root / "class/dmi/id"
        dmi.mkdir(parents=True)
        if vendor is not None:
            (dmi / "sys_vendor").write_text(vendor + "\n")
        if product is not None:
            (dmi / "product_name").write_text(product + "\n")
        if efi:
            (root / "firmware/efi").mkdir(parents=True)
        return root

    def check(self, root):
        return subprocess.run(
            ["sh", "-c", '. "$1"; locasos_guard_check "$2"',
             "guard-test", str(GUARD), str(root)],
            capture_output=True, text=True, timeout=5,
        )

    def test_supported_uefi_virtual_machines(self):
        for vendor, product in (
            ("Microsoft Corporation", "Virtual Machine"),
            ("QEMU", "Standard PC (Q35 + ICH9, 2009)"),
            ("QEMU", "Standard PC (i440FX + PIIX, 1996)"),
        ):
            with self.subTest(vendor=vendor, product=product), tempfile.TemporaryDirectory() as tmp:
                self.assertEqual(self.check(self.fixture(tmp, vendor, product)).returncode, 0)

    def test_unknown_missing_or_near_match_identity_is_refused(self):
        for vendor, product in (
            (None, "Virtual Machine"), ("QEMU", None),
            ("Dell Inc.", "Virtual Machine"),
            ("Microsoft Corporation", "Physical Machine"),
            ("Microsoft Corporation Spoof", "Virtual Machine"),
            ("QEMU", "unknown"), ("KVM", "Standard PC (Q35 + ICH9, 2009)"),
        ):
            with self.subTest(vendor=vendor, product=product), tempfile.TemporaryDirectory() as tmp:
                self.assertNotEqual(self.check(self.fixture(tmp, vendor, product)).returncode, 0)

    def test_legacy_bios_is_refused_even_with_supported_dmi(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertNotEqual(self.check(self.fixture(tmp, efi=False)).returncode, 0)

    def test_cli_entrypoint_uses_real_sysfs_not_fixture_argument(self):
        # Replace only the sourced check/refusal functions in this test process.
        # Main must pass the literal /sys and refuse any CLI arguments.
        command = '''. "$1"
locasos_guard_check() { [ "$#" -eq 1 ] && [ "$1" = /sys ]; }
locasos_guard_refuse() { exit 73; }
locasos_guard_main
'''
        result = subprocess.run(["sh", "-c", command, "guard-test", str(GUARD)],
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)
        command += 'locasos_guard_main /tmp/fake-sys\n'
        result = subprocess.run(["sh", "-c", command, "guard-test", str(GUARD)],
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 73)

    def test_refusal_never_returns_and_ignores_interrupts(self):
        process = subprocess.Popen(
            ["sh", "-c", '. "$1"; locasos_guard_refuse; echo UNSAFE_RETURN',
             "guard-test", str(GUARD)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, start_new_session=True,
        )
        try:
            self.assertIn("LOCASOS_INSTALLER_REFUSED", process.stderr.readline())
            for signum in (signal.SIGINT, signal.SIGTERM):
                os.killpg(process.pid, signum)
                with self.assertRaises(subprocess.TimeoutExpired):
                    process.wait(timeout=0.1)
        finally:
            os.killpg(process.pid, signal.SIGKILL)
            stdout, _ = process.communicate(timeout=5)
        self.assertNotIn("UNSAFE_RETURN", stdout)


if __name__ == "__main__":
    unittest.main()
