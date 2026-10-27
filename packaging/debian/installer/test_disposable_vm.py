import importlib.util
import pathlib
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("disposable_vm", pathlib.Path(__file__).with_name("disposable_vm.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class DisposableVMTests(unittest.TestCase):
    def test_commands_use_scratch_qcow2_not_host_block_device(self):
        create, vm = module.build_commands(
            pathlib.Path("/tmp/locasos.iso"),
            pathlib.Path("/tmp/random/disposable.qcow2"), None)
        self.assertEqual(create[:4], ["qemu-img", "create", "-f", "qcow2"])
        self.assertIn("format=qcow2", " ".join(vm))
        self.assertIn("-net", vm)
        self.assertIn("none", vm)
        self.assertFalse(any(a.startswith("/dev/") for a in create + vm))
    def test_size_and_ram_bounded(self):
        for size in (0, 63, 1025):
            with self.assertRaises(ValueError):
                module.build_commands(pathlib.Path("/tmp/test.iso"), pathlib.Path("/tmp/disk"), None, disk_gib=size)
        with self.assertRaises(ValueError):
            module.build_commands(pathlib.Path("/tmp/test.iso"), pathlib.Path("/tmp/disk"), None, memory_mb=0)
    def test_readonly_firmware_and_media(self):
        _, args = module.build_commands(pathlib.Path("/tmp/test.iso"), pathlib.Path("/tmp/disk"), pathlib.Path("/tmp/OVMF_CODE.fd"))
        self.assertTrue(any("readonly=on" in arg for arg in args))
        self.assertTrue(any("if=pflash" in arg and "readonly=on" in arg for arg in args))
    def test_checksum(self):
        with tempfile.TemporaryDirectory() as folder:
            testfile = pathlib.Path(folder) / "test.iso"
            testfile.write_bytes(b"test")
            self.assertEqual(module.checksum(testfile), "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08")

if __name__ == "__main__":
    unittest.main()
