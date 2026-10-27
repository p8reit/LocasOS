import importlib.util
import unittest
from pathlib import Path

path = Path(__file__).with_name("vm_install_integration.py")
spec = importlib.util.spec_from_file_location("vm_install_integration", path)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

class IntegrationHarnessTests(unittest.TestCase):
    def test_first_boot_uses_only_scratch_disk_and_explicit_marker(self):
        cmd = mod.qemu_args(Path("/tmp/live.iso"), Path("/tmp/only-disk.qcow2"), Path("/tmp/OVMF_CODE.fd"),Path("/tmp/OVMF_VARS.fd"),True)
        self.assertIn("name=opt/locasos/ephemeral,string=locasos-ephemeral-disk-v1",cmd)
        self.assertIn("type=1,manufacturer=QEMU,product=LocasOS-QEMU-InstallTest",cmd)
        self.assertIn("file=/tmp/only-disk.qcow2,format=qcow2,cache=none",cmd)
        self.assertIn("readonly=on", " ".join(cmd))
        self.assertNotIn("-hda",cmd)
        self.assertNotIn("-usb",cmd)
    def test_second_boot_removes_iso_and_destructive_marker(self):
        cmd = mod.qemu_args(Path("/tmp/live.iso"),Path("/tmp/only-disk.qcow2"),Path("/tmp/OVMF_CODE.fd"),Path("/tmp/OVMF_VARS.fd"),False)
        text = " ".join(cmd)
        self.assertNotIn("fw_cfg",text)
        self.assertNotIn("live.iso",text)
        self.assertIn("order=c",text)
    def test_harness_cannot_use_network(self):
        cmd = mod.qemu_args(Path("/tmp/live.iso"),Path("/tmp/only-disk.qcow2"),Path("/tmp/OVMF_CODE.fd"),Path("/tmp/OVMF_VARS.fd"),True)
        self.assertEqual(cmd[cmd.index("-net")+1],"none")

if __name__=="__main__": unittest.main()
