import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("storage_preflight", Path(__file__).with_name("storage_preflight.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

def disk(**overrides):
    item = dict(name="sda", path="/dev/sda", type="disk", size=1000000000, model="Test", serial="123", tran="sata", rm=False, ro=False, fstype=None, mountpoints=[None])
    item.update(overrides)
    return item

class StoragePreflightTests(unittest.TestCase):
    def test_blank_disk_is_candidate_but_never_approved_for_writing(self):
        result = module.report({"blockdevices":[disk()]})
        self.assertTrue(result["disks"][0]["candidate"])
        self.assertFalse(result["installation_enabled"])
    def test_existing_partitions_are_blocked_even_if_unmounted(self):
        child = dict(name="sda1",type="part",fstype=None,mountpoints=[None])
        item = module.assess(disk(children=[child]))
        self.assertIn("existing_partitions_or_devices",item["blockers"])
    def test_mounted_root_partition_is_blocked(self):
        child = dict(name="sda1",type="part",fstype="ext4",mountpoints=["/"])
        item = module.assess(disk(children=[child]))
        self.assertIn("mounted_filesystem",item["blockers"])
        self.assertIn("existing_filesystem",item["blockers"])
    def test_live_usb_is_blocked(self):
        item = module.assess(disk(rm=True,children=[dict(name="sda1",type="part",mountpoints=["/run/live/medium"])]))
        self.assertFalse(item["candidate"])
    def test_read_only_and_invalid_size_are_blocked(self):
        item = module.assess(disk(ro=1,size=0))
        self.assertIn("read_only",item["blockers"])
        self.assertIn("invalid_size",item["blockers"])
    def test_non_disk_and_missing_device_path_are_blocked(self):
        item = module.assess(disk(type="loop",path="not-a-device"))
        self.assertFalse(item["candidate"])
    def test_multiple_disks_classified_independently(self):
        result = module.report({"blockdevices":[disk(),disk(name="nvme0n1",path="/dev/nvme0n1",children=[dict(type="part",fstype="vfat",mountpoints=["/boot/efi"])])]})
        self.assertEqual([x["candidate"] for x in result["disks"]],[True,False])
if __name__ == "__main__":
    unittest.main()
