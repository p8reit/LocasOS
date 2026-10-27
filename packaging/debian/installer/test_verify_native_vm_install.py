import importlib.util
import pathlib
import unittest

root = pathlib.Path(__file__).parent
spec = importlib.util.spec_from_file_location("verify_native_vm_install", root / "verify_native_vm_install.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

class VMInstallSafetyTests(unittest.TestCase):
    def test_qemu_install_attaches_only_scratch_image(self):
        args = m.vm_cmd(pathlib.Path("/tmp/code.fd"), pathlib.Path("/tmp/vars.fd"),
                        pathlib.Path("/tmp/scratch.qcow2"), pathlib.Path("/tmp/live.iso"))
        self.assertIn("name=opt/locasos/dev-install,string=LOCASOS_DISPOSABLE_VM_INSTALL_V1", args)
        self.assertFalse(any("/dev/sd" in x or "/dev/nvme" in x for x in args))
        self.assertIn("scsi-hd,drive=scratch,serial=locasos-scratch", args)
    def test_live_health_boot_cannot_trigger_disk_install(self):
        args = m.vm_cmd(pathlib.Path("/tmp/code.fd"), pathlib.Path("/tmp/vars.fd"),
                        pathlib.Path("/tmp/scratch.qcow2"), pathlib.Path("/tmp/live.iso"), enable_install=False)
        self.assertFalse(any("fw_cfg" in x or "DISPOSABLE_VM_INSTALL" in x for x in args))
        self.assertTrue(any("readonly=on" in x and "live.iso" in x for x in args))
    def test_second_boot_has_no_install_marker_or_iso(self):
        args = m.vm_cmd(pathlib.Path("/tmp/code.fd"), pathlib.Path("/tmp/vars.fd"),
                        pathlib.Path("/tmp/scratch.qcow2"))
        self.assertFalse(any("fw_cfg" in x or "live.iso" in x for x in args))
        self.assertIn("order=c", args)
    def test_guest_installer_requires_strict_opt_in_and_blank_disk(self):
        content = (root / "install-qemu-only.sh").read_text()
        for required in ("QEMU opt-in marker missing", "locasos-scratch", "disk contains partitions",
                         "existing filesystem", "mounted disk", "LOCASOS_DISPOSABLE_VM_INSTALL_V1"):
            self.assertIn(required, content)
        self.assertNotIn(" /dev/sda\n", content)
if __name__ == "__main__": unittest.main()
