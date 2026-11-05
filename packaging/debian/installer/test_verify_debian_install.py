"""Host containment and fresh guest-evidence tests; these do not emulate a VM."""
import gzip
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "verify_debian_install", Path(__file__).with_name("verify_debian_install.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class VMCommands(unittest.TestCase):
    def command(self, **kwargs):
        return m.vm_command(Path("/tmp/code.fd"), Path("/tmp/vars.fd"),
                            Path("/tmp/scratch.qcow2"), Path("/tmp/monitor.sock"), **kwargs)

    def test_installer_has_only_readonly_iso_and_scratch_disk(self):
        args = self.command(iso=Path("/tmp/image.iso"), kernel=Path("/tmp/vmlinuz"),
                            initrd=Path("/tmp/ci-initrd.gz"))
        self.assertIn("scsi-hd,drive=scratch,serial=locasos-scratch", args)
        self.assertIn("name=opt/locasos/dev-install,string=" + m.TOKEN, args)
        self.assertIn("if=none,id=cd,format=raw,readonly=on,media=cdrom,file=/tmp/image.iso", args)
        self.assertEqual(args[args.index("-net") + 1], "none")
        self.assertFalse(any("file=/dev/" in arg or "vfio" in arg or "virtfs" in arg for arg in args))

    def test_disk_boot_has_no_install_media_or_external_kernel(self):
        args = self.command()
        for forbidden in ("-kernel", "-initrd", "-append", "-fw_cfg"):
            self.assertNotIn(forbidden, args)
        self.assertFalse(any("image.iso" in arg or "media=cdrom" in arg for arg in args))
        self.assertIn("order=c", args)

    def test_partial_installer_parameters_rejected(self):
        with self.assertRaisesRegex(ValueError, "kernel"):
            self.command(iso=Path("/tmp/image.iso"))
        with self.assertRaisesRegex(ValueError, "disk-only"):
            self.command(kernel=Path("/tmp/vmlinuz"))

    def test_missing_tool_refuses_before_vm_creation(self):
        with mock.patch.object(m.shutil, "which", return_value=None):
            with self.assertRaisesRegex(ValueError, "xorriso missing"):
                m.validate_inputs("not-read", "0" * 64, "not-read", "not-read")

    def test_digest_mismatch_refuses_exact_iso(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "test.iso"
            path.write_bytes(b"exact image")
            with mock.patch.object(m.shutil, "which", return_value="/mock/tool"):
                with self.assertRaisesRegex(ValueError, "digest mismatch"):
                    m.validate_inputs(path, "0" * 64, path, path)
                self.assertEqual(m.validate_inputs(path, m.checksum(path), path, path)[0], path)

    def test_qemu_option_separator_and_nonfile_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "test,format=host_device"
            path.touch()
            with self.assertRaisesRegex(ValueError, "regular file"):
                m.checked_file(path)
            with self.assertRaisesRegex(ValueError, "regular file"):
                m.checked_file(root)


class InitrdContainment(unittest.TestCase):
    def test_test_preseed_is_appended_only_to_copy(self):
        with tempfile.TemporaryDirectory() as root:
            source, dest = Path(root) / "original.gz", Path(root) / "ci.gz"
            original = gzip.compress(b"original-initramfs", mtime=0)
            source.write_bytes(original)
            m.prepare_initrd(source, dest)
            self.assertEqual(source.read_bytes(), original)
            self.assertTrue(dest.read_bytes().startswith(original))
            unpacked = gzip.decompress(dest.read_bytes())
            self.assertIn(b"070701", unpacked)
            self.assertIn(b"preseed.cfg\0", unpacked)
            self.assertIn(m.CI_GUARD.encode(), unpacked)

    def test_unsafe_initrd_path_rejected(self):
        for name in ("/etc/passwd", "../etc/passwd"):
            with self.subTest(name=name):
                with self.assertRaisesRegex(ValueError, "unsafe"):
                    m.newc_archive([(name, b"", 0o100644)])

    def test_unattended_settings_are_guarded_and_test_only(self):
        self.assertIn("/usr/lib/locasos/ci-guard.sh", m.PRESEED)
        for required in ("qemu_fw_cfg", "locasos-scratch", m.TOKEN,
                         "expected exactly one scratch disk", "disk contains partitions",
                         "mounted disk", "blkid unavailable", "poweroff -f"):
            self.assertIn(required, m.CI_GUARD)
        self.assertNotIn("partman-auto/disk string /dev/", m.PRESEED)
        self.assertLess(m.CI_GUARD.index("qemu_fw_cfg"), m.CI_GUARD.index("db_set partman-auto/disk"))
        self.assertIn("passwd/user-password-crypted password !", m.PRESEED)
        self.assertIn("/target/etc/locasos-installed-vm-test", m.CI_FINISH)


class FreshEvidence(unittest.TestCase):
    def run_process(self, body, stage="install"):
        with tempfile.TemporaryDirectory() as root:
            log = Path(root) / "serial.log"
            log.write_text("LOCASOS_VM_INSTALL_COMPLETE\n")
            m.run_stage([sys.executable, "-c", body], stage, 3,
                        Path(root) / "monitor.sock", log)
            return log.read_text()

    def test_clean_installer_exit_requires_exact_marker(self):
        self.assertIn("LOCASOS_VM_INSTALL_COMPLETE", self.run_process(
            "print('LOCASOS_VM_INSTALL_COMPLETE', flush=True)"))

    def test_old_log_and_substring_never_satisfy_stage(self):
        with self.assertRaisesRegex(RuntimeError, "without successful evidence"):
            self.run_process("print('echo LOCASOS_VM_INSTALL_COMPLETE', flush=True)")

    def test_guest_failure_after_success_is_rejected(self):
        with self.assertRaises(RuntimeError):
            self.run_process("print('LOCASOS_VM_INSTALL_COMPLETE\\nLOCASOS_VM_INSTALL_FAIL: test', flush=True)")

    def test_nonzero_exit_after_marker_is_rejected(self):
        with self.assertRaisesRegex(RuntimeError, "exit 2"):
            self.run_process("import sys; print('LOCASOS_VM_INSTALL_COMPLETE', flush=True); sys.exit(2)")

    def test_installed_boot_requests_clean_guest_poweroff(self):
        with mock.patch.object(m.socket, "socket") as connection:
            self.run_process("import time; print('LOCASOS_INSTALLED_BOOT_OK', flush=True); time.sleep(.05)",
                             stage="installed")
            connection.return_value.__enter__.return_value.sendall.assert_called_once_with(b"system_powerdown\n")


if __name__ == "__main__":
    unittest.main()
