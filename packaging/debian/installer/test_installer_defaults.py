"""The distributed installer must retain the operator's destructive decisions."""
from pathlib import Path
import unittest


class InstallerDefaults(unittest.TestCase):
    def test_only_non_destructive_defaults_are_preseeded(self):
        source = Path(__file__).with_name("preseed.cfg").read_text()
        entries = [line.split(None, 3) for line in source.splitlines()
                   if line.strip() and not line.lstrip().startswith("#")]
        allowed = {"preseed/early_command", "preseed/late_command",
                   "live-installer/enable", "live-installer/mode", "passwd/root-login"}
        self.assertEqual({entry[1] for entry in entries}, allowed)
        self.assertTrue(all(entry[0] == "d-i" for entry in entries))
        values = {entry[1]: entry[3] for entry in entries}
        self.assertEqual(values["preseed/early_command"], "/usr/lib/locasos/guard-vm.sh")
        self.assertEqual(values["preseed/late_command"], "/usr/lib/locasos/finish-install.sh")
        self.assertEqual(values["passwd/root-login"], "false")

    def test_readiness_does_not_block_multi_user_target(self):
        source = Path(__file__).with_name("locasos-guest-readiness.service").read_text()
        self.assertIn("Type=simple\n", source)
        self.assertIn("RuntimeMaxSec=300\n", source)
        self.assertNotIn("Type=oneshot", source)


if __name__ == "__main__":
    unittest.main()
