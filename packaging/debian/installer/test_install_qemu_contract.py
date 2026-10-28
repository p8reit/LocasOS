"""VM-only guest installer contract checks; never execute destructive commands in CI."""
from pathlib import Path
import unittest


SCRIPT = Path(__file__).with_name("install-qemu-only.sh").read_text(encoding="utf-8")


class GuestInstallerContractTests(unittest.TestCase):
    def test_identity_precedes_first_mutation(self):
        first_write = SCRIPT.index('sgdisk --clear "$disk"')
        for check in ('qemu_fw_cfg', 'LOCASOS_DISPOSABLE_VM_INSTALL_V1',
                      'locasos-scratch', 'VM target changed after preflight',
                      'locasos-vm-target-check'):
            with self.subTest(check=check):
                self.assertLess(SCRIPT.index(check), first_write)

    def test_persistent_data_sentinel_and_fstab(self):
        self.assertIn('UUID=$data_uuid /var/lib/locas', SCRIPT)
        self.assertIn('mount "$data" /mnt/locasos-target/var/lib/locas', SCRIPT)
        self.assertIn('.locasos-vm-install-sentinel', SCRIPT)
        self.assertIn('locasos-vm-install-v1', SCRIPT)

    def test_no_physical_device_or_network_entrypoint(self):
        self.assertNotIn('--device', SCRIPT)
        self.assertNotIn('/dev/nvme', SCRIPT)
        self.assertNotIn('mkfs.ext4 /dev/sda', SCRIPT)
        self.assertNotIn('curl ', SCRIPT)
        self.assertNotIn('wget ', SCRIPT)


if __name__ == "__main__":
    unittest.main()
