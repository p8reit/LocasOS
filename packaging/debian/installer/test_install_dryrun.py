import importlib.util
import unittest
from pathlib import Path
spec = importlib.util.spec_from_file_location("install_dryrun", Path(__file__).with_name("install_dryrun.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
ID="/dev/disk/by-id/virtio-test-disk"
def fixture(**kwargs):
    disk=dict(name="vda",path="/dev/vda",type="disk",size=128*module.GIB,
              model="QEMU TEST",serial="fixture",tran="virtio",rm=0,ro=0,mouse=None,mountpoints=[None],fstype=None)
    disk.update(kwargs)
    return {"blockdevices":[disk],"identities":{"/dev/vda":[ID]}}
class DryRunTests(unittest.TestCase):
    def test_blank_disk_proposal_never_authorizes_writes(self):
        f=fixture()
        p=module.plan(f,ID,f["identities"])
        self.assertEqual(p["blockers"],[])
        self.assertFalse(p["destructive_operations_enabled"])
        self.assertGreater(p["layout_proposal"][-1]["size_bytes"],0)
    def test_wrong_or_ambiguous_identity_refused(self):
        f=fixture()
        with self.assertRaises(ValueError): module.plan(f,"/dev/vda",f["identities"])
        with self.assertRaises(ValueError): module.plan(f,"/dev/disk/by-id/wrong",f["identities"])
        f["blockdevices"].append(dict(f["blockdevices"][0],path="/dev/vdb"))
        f["identities"]["/dev/vdb"]=[ID]
        with self.assertRaises(ValueError): module.plan(f,ID,f["identities"])
    def test_too_small_disk_blocked(self):
        f=fixture(size=32*module.GIB)
        p=module.plan(f,ID,f["identities"])
        self.assertIn("insufficient_capacity_64_gib",p["blockers"])
    def test_existing_partitions_and_mounts_blocked(self):
        f=fixture(children=[dict(type="part",fstype="ext4",mountpoints=["/"])])
        p=module.plan(f,ID,f["identities"])
        self.assertIn("mounted_filesystem",p["blockers"])
        self.assertIn("existing_partitions_or_devices",p["blockers"])
    def test_removable_disk_blocked(self):
        f=fixture(rm=True)
        self.assertIn("removable_media",module.plan(f,ID,f["identities"])["blockers"])
if __name__=="__main__": unittest.main()
