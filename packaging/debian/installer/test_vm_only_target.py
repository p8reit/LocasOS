import importlib.util
from pathlib import Path
import unittest

p = Path(__file__).with_name("vm_only_target.py")
spec = importlib.util.spec_from_file_location("vm_only_target", p)
module = importlib.util.module_from_spec(spec)
import sys
sys.modules[spec.name] = module
spec.loader.exec_module(module)


def fixture():
    return {"blockdevices": [{
        "name": "sda", "path": "/dev/sda", "type": "disk",
        "model": "QEMU HARDDISK", "serial": "locasos-scratch",
        "size": 64 * 1024 ** 3, "ro": 0, "rm": 0,
        "fstype": None, "mountpoints": [None], "children": [],
    }, {"name": "sr0", "type": "rom", "mountpoints": [None]}]}


def check(d, vendor="QEMU", marker="LOCASOS_DISPOSABLE_VM_INSTALL_V1"):
    return module.verify_vm_target(d, vendor=vendor, firmware_marker=marker)


class VMTargetTests(unittest.TestCase):
    def test_accepts_exact_disposable_virtual_disk(self):
        v = check(fixture())
        self.assertEqual(v.path, "/dev/sda")
        self.assertEqual(v.serial, "locasos-scratch")

    def test_missing_or_fake_vm_opt_in_rejected(self):
        for name, marker in [("QEMU Corporation", "LOCASOS_DISPOSABLE_VM_INSTALL_V1"),
                             ("QEMU", ""), ("QEMU", "MALICIOUS")]:
            with self.subTest(name=name, marker=marker), self.assertRaises(module.UnsafeTarget):
                check(fixture(), name, marker)

    def test_wrong_serial_and_model_rejected(self):
        for key, value in [("serial", "host-drive"), ("model", "Samsung SSD")]:
            data = fixture()
            data["blockdevices"][0][key] = value
            with self.assertRaises(module.UnsafeTarget):
                check(data)

    def test_second_disk_and_partitions_rejected(self):
        data = fixture()
        data["blockdevices"].append({**data["blockdevices"][0], "name": "sdb", "path": "/dev/sdb"})
        with self.assertRaises(module.UnsafeTarget):
            check(data)
        data = fixture()
        data["blockdevices"][0]["children"] = [{"type": "part", "fstype": "ext4"}]
        with self.assertRaises(module.UnsafeTarget):
            check(data)

    def test_mounted_or_formatted_disk_rejected(self):
        for key, value in [("fstype", "ext4"), ("mountpoints", ["/data"]), ("ro", 1), ("rm", 1)]:
            data = fixture()
            data["blockdevices"][0][key] = value
            with self.assertRaises(module.UnsafeTarget):
                check(data)

    def test_bad_sizes_paths_and_inventory_rejected(self):
        for key, value in [("size", 0), ("size", 63 * 1024**3),
                           ("size", 129 * 1024**3), ("size", "68719476736"),
                           ("path", "/dev/nvme0n1"), ("path", "/dev/sda1"),
                           ("path", "/dev/sdaa")]:
            data = fixture()
            data["blockdevices"][0][key] = value
            with self.assertRaises(module.UnsafeTarget):
                check(data)
        with self.assertRaises(module.UnsafeTarget):
            check({"blockdevices": []})


if __name__ == "__main__":
    unittest.main()
