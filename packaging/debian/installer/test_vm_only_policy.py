"""Regression tests for non-destructive native VM source policy."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

source = Path(__file__).with_name("check_vm_only_policy.py")
spec = importlib.util.spec_from_file_location("check_vm_only_policy", source)
policy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(policy)

GOOD_SCRIPT = '''#!/bin/bash
[[ -r /sys/firmware/qemu_fw_cfg/by_name/opt/locasos/dev-install/raw ]] || exit 2
[[ $(cat /sys/firmware/qemu_fw_cfg/by_name/opt/locasos/dev-install/raw) == LOCASOS_DISPOSABLE_VM_INSTALL_V1 ]] || exit 2
[[ -e /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_locasos-scratch ]] || exit 2
[[ $(lsblk -nr -o FSTYPE "$disk") == "" ]] || exit 2
sgdisk --clear "$disk"
'''


class SafetyGateTests(unittest.TestCase):
    def scan(self, files):
        with tempfile.TemporaryDirectory() as folder:
            p = Path(folder)
            for name, body in files.items():
                (p / name).write_text(body, encoding="utf-8")
            return policy.violations(p)

    def test_read_only_sources_permitted(self):
        self.assertEqual(self.scan({"inventory.sh": "lsblk --json\n"}), [])

    def test_single_qemu_guest_installer_permitted(self):
        self.assertEqual(self.scan({"install-qemu-only.sh": GOOD_SCRIPT}), [])

    def test_reject_other_destructive_scripts(self):
        result = self.scan({"other.sh": "sgdisk --clear /dev/sda\n"})
        self.assertTrue(any("outside isolated guest allowlist" in e for e in result))

    def test_reject_explicit_hardware_disk(self):
        result = self.scan({"install-qemu-only.sh": GOOD_SCRIPT + "mkfs.ext4 /dev/sda1\n"})
        self.assertTrue(any("hard-coded" in e for e in result))

    def test_reject_preflight_after_mutation(self):
        result = self.scan({"install-qemu-only.sh": 'sgdisk --clear "$disk"\n' + GOOD_SCRIPT})
        self.assertTrue(any("precede first disk write" in e for e in result))

    def test_reject_two_destructive_installers(self):
        result = self.scan({"install-qemu-only.sh": GOOD_SCRIPT,
                            "install-ephemeral-guest.sh": GOOD_SCRIPT})
        self.assertTrue(any("competing destructive" in e for e in result))

    def test_reject_host_passthrough(self):
        result = self.scan({"runner.sh": "qemu-system-x86_64 -drive file=/dev/sdb\n"})
        self.assertTrue(any("host device passthrough" in e for e in result))


if __name__ == "__main__":
    unittest.main()
