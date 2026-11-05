"""Host containment and fresh guest-evidence tests; these do not emulate a VM."""
import gzip
import importlib.util
from pathlib import Path
import sys
import subprocess
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
        self.assertEqual(args[args.index("-uuid") + 1], m.DEFAULT_FIRMWARE_UUID)
        self.assertNotIn("-fw_cfg", args)
        self.assertIn("if=none,id=cd,format=raw,readonly=on,media=cdrom,file=/tmp/image.iso", args)
        self.assertEqual(args[args.index("-net") + 1], "none")
        self.assertFalse(any("file=/dev/" in arg or "vfio" in arg or "virtfs" in arg for arg in args))

    def test_disk_boot_has_no_install_media_or_external_kernel(self):
        args = self.command()
        for forbidden in ("-kernel", "-initrd", "-append", "-fw_cfg", "-uuid"):
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


class FirmwareOptIn(unittest.TestCase):
    def test_per_run_uuid_bound_to_guest_guard_and_qemu(self):
        expected = "3d46da63-7207-431e-aea4-4d25bc739c83"
        guard = m.render_ci_guard(expected)
        self.assertIn(expected, guard)
        self.assertNotIn(m.DEFAULT_FIRMWARE_UUID, guard)
        args = m.vm_command(Path("/tmp/code.fd"), Path("/tmp/vars.fd"), Path("/tmp/scratch.qcow2"),
                            Path("/tmp/monitor.sock"), Path("/tmp/image.iso"),
                            Path("/tmp/vmlinuz"), Path("/tmp/ci-initrd.gz"), firmware_uuid=expected)
        self.assertEqual(args[args.index("-uuid") + 1], expected)

    def test_invalid_or_noncanonical_uuid_never_reaches_shell(self):
        for invalid in ("", "not-a-uuid", "89d183da0283468e9dbf965ebf5e66ed",
                        m.DEFAULT_FIRMWARE_UUID.upper(), "'; poweroff; #", None):
            with self.subTest(value=invalid):
                with self.assertRaisesRegex(ValueError, "canonical firmware UUID"):
                    m.render_ci_guard(invalid)

    def run_uuid_guard(self, actual):
        with tempfile.TemporaryDirectory() as root:
            marker = Path(root) / "product_uuid"
            if actual is not None:
                marker.write_text(actual + "\n")
            snippet = m.firmware_uuid_guard(m.DEFAULT_FIRMWARE_UUID).replace(
                "/sys/class/dmi/id/product_uuid", str(marker))
            return subprocess.run(["sh", "-c", 'refuse() { echo "$*"; exit 7; };\n' + snippet],
                                  capture_output=True, text=True)

    def test_guest_matching_uuid_accepts_firmware_case(self):
        self.assertEqual(self.run_uuid_guard(m.DEFAULT_FIRMWARE_UUID).returncode, 0)
        self.assertEqual(self.run_uuid_guard(m.DEFAULT_FIRMWARE_UUID.upper()).returncode, 0)

    def test_guest_wrong_or_missing_uuid_refuses(self):
        for actual in (None, "3d46da63-7207-431e-aea4-4d25bc739c83", "garbage"):
            with self.subTest(actual=actual):
                result = self.run_uuid_guard(actual)
                self.assertEqual(result.returncode, 7)
                self.assertIn("SMBIOS install UUID", result.stdout)


class InitrdContainment(unittest.TestCase):
    def test_unexpected_preflight_error_refuses_instead_of_returning(self):
        preamble = m.render_ci_guard().split("/usr/lib/locasos/guard-vm.sh", 1)[0]
        # Replace shutdown only; exercise the real shell error trap without
        # running any device inventory or power command on the test host.
        result = subprocess.run(["sh", "-c", preamble +
            '\nrefuse() { trap - 0; exit 77; }\nfalse\necho unsafe-return\n'],
            capture_output=True, text=True, timeout=3)
        self.assertEqual(result.returncode, 77)
        self.assertNotIn("unsafe-return", result.stdout)

    def production_archive(self, omitted=(), settings=None, mode=0o100755):
        files = [("preseed.cfg", (settings or (
            "d-i preseed/early_command string /usr/lib/locasos/guard-vm.sh\n"
            "d-i preseed/late_command string /usr/lib/locasos/finish-install.sh\n")).encode(), 0o100644),
            ("usr/lib/locasos/guard-vm.sh", b"#!/bin/sh\nexit 0\n", mode),
            ("usr/lib/locasos/finish-install.sh", b"#!/bin/sh\nexit 0\n", mode)]
        return m.newc_archive([entry for entry in files if entry[0] not in omitted])

    def validate_fixture(self, archive):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "installer.gz"
            path.write_bytes(gzip.compress(archive, mtime=0))
            m.validate_production_initrd(path)

    def test_test_preseed_is_appended_only_to_copy(self):
        with tempfile.TemporaryDirectory() as root:
            source, dest = Path(root) / "original.gz", Path(root) / "ci.gz"
            original = gzip.compress(self.production_archive(), mtime=0)
            source.write_bytes(original)
            m.prepare_initrd(source, dest)
            self.assertEqual(source.read_bytes(), original)
            self.assertTrue(dest.read_bytes().startswith(original))
            unpacked = gzip.decompress(dest.read_bytes())
            self.assertIn(b"070701", unpacked)
            self.assertIn(b"preseed.cfg\0", unpacked)
            self.assertIn(m.render_ci_guard().encode(), unpacked)

    def test_temporary_initrd_embeds_the_same_per_run_uuid(self):
        expected = "3d46da63-7207-431e-aea4-4d25bc739c83"
        with tempfile.TemporaryDirectory() as root:
            source, dest = Path(root) / "original.gz", Path(root) / "ci.gz"
            source.write_bytes(gzip.compress(self.production_archive(), mtime=0))
            m.prepare_initrd(source, dest, firmware_uuid=expected)
            guard = m.initrd_members(dest)["usr/lib/locasos/ci-guard.sh"][1].decode()
            self.assertIn(expected, guard)
            self.assertNotIn(m.DEFAULT_FIRMWARE_UUID, guard)
            self.assertNotIn("__LOCASOS_FIRMWARE_UUID_GUARD__", guard)

    def test_shipped_initrd_missing_guard_or_finish_refused(self):
        for name in ("preseed.cfg", "usr/lib/locasos/guard-vm.sh", "usr/lib/locasos/finish-install.sh"):
            with self.subTest(member=name):
                with self.assertRaisesRegex(ValueError, "missing"):
                    self.validate_fixture(self.production_archive(omitted=(name,)))

    def test_nonexecutable_shipped_hooks_refused(self):
        with self.assertRaisesRegex(ValueError, "not executable"):
            self.validate_fixture(self.production_archive(mode=0o100644))

    def test_shipped_preseed_must_wire_both_commands(self):
        with self.assertRaisesRegex(ValueError, "safety command missing"):
            self.validate_fixture(self.production_archive(settings="d-i live-installer/enable boolean true\n"))

    def test_later_preseed_command_cannot_override_required_guard(self):
        settings = ("d-i preseed/early_command string /usr/lib/locasos/guard-vm.sh\n"
                    "d-i preseed/early_command string true\n"
                    "d-i preseed/late_command string /usr/lib/locasos/finish-install.sh\n")
        with self.assertRaisesRegex(ValueError, "overridden"):
            self.validate_fixture(self.production_archive(settings=settings))

    def test_shipped_preseed_cannot_automate_partitioning(self):
        settings = ("d-i preseed/early_command string /usr/lib/locasos/guard-vm.sh\n"
                    "d-i preseed/late_command string /usr/lib/locasos/finish-install.sh\n"
                    "d-i partman/confirm boolean true\n")
        with self.assertRaisesRegex(ValueError, "must not automate"):
            self.validate_fixture(self.production_archive(settings=settings))

    def test_truncated_newc_refused(self):
        for raw in (b"070701", self.production_archive()[:150]):
            with self.subTest(size=len(raw)):
                with self.assertRaisesRegex(ValueError, "invalid|truncated"):
                    self.validate_fixture(raw)

    def test_concatenated_gzip_archives_use_final_member(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "installer.gz"
            valid = gzip.compress(self.production_archive(), mtime=0)
            override = m.newc_archive([("preseed.cfg", b"bad override", 0o100644)])
            path.write_bytes(valid + gzip.compress(b"\0" * 512 + override, mtime=0))
            with self.assertRaisesRegex(ValueError, "safety command missing"):
                m.validate_production_initrd(path)

    def test_unsafe_initrd_path_rejected(self):
        for name in ("/etc/passwd", "../etc/passwd"):
            with self.subTest(name=name):
                with self.assertRaisesRegex(ValueError, "unsafe"):
                    m.newc_archive([(name, b"", 0o100644)])

    def test_unattended_settings_are_guarded_and_test_only(self):
        self.assertIn("d-i preseed/early_command string /usr/lib/locasos/guard-vm.sh", m.PRESEED)
        self.assertIn("d-i partman/early_command string /usr/lib/locasos/ci-guard.sh", m.PRESEED)
        guard = m.render_ci_guard()
        for required in ("/sys/class/dmi/id/product_uuid", "locasos-scratch", m.DEFAULT_FIRMWARE_UUID,
                         "expected exactly one scratch disk", "disk contains partitions",
                         "mounted disk", "blkid unavailable", "poweroff -f", "udevadm settle",
                         "udevadm info --query=property", "ID_SERIAL_SHORT=locasos-scratch",
                         "ID_VENDOR=QEMU", "ID_MODEL=QEMU_HARDDISK"):
            self.assertIn(required, guard)
        self.assertNotIn("/device/serial", guard)
        self.assertNotIn("qemu_fw_cfg", guard)
        self.assertLess(guard.index("udevadm settle"), guard.index("list-devices disk"))
        self.assertNotIn("partman-auto/disk string /dev/", m.PRESEED)
        self.assertLess(guard.index("/sys/class/dmi/id/product_uuid"), guard.index("db_set partman-auto/disk"))
        self.assertIn("passwd/user-password-crypted password !", m.PRESEED)
        self.assertIn("/target/etc/locasos-installed-vm-test", m.CI_FINISH)



class BootMenuWiring(unittest.TestCase):
    ENTRY = "menuentry 'Install LocasOS' {\n linux /install/vmlinuz vga=normal --- quiet\n initrd /install/initrd.gz\n}\n"

    def check(self, root="source /boot/grub/install_start.cfg\n", installer=None, extras=None):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp)
            (path / "grub.cfg").write_text(root)
            if installer is not None:
                (path / "install_start.cfg").write_text(installer)
            for name, body in (extras or {}).items():
                (path / name).write_text(body)
            m.validate_boot_menu(path)

    def test_live_build_installer_include_is_wired(self):
        self.check(installer=self.ENTRY)

    def test_inline_installer_is_wired(self):
        self.check(root=self.ENTRY)

    def test_missing_installer_menu_or_include_refused(self):
        for root in ("# source /boot/grub/install_start.cfg\n", "source /boot/grub/install_start.cfg\n"):
            with self.subTest(root=root):
                with self.assertRaisesRegex(ValueError, "no wired|include missing"):
                    self.check(root=root)

    def test_disabled_installer_include_refused(self):
        with self.assertRaisesRegex(ValueError, "disabled"):
            self.check(root="if false; then\nsource /boot/grub/install_start.cfg\nfi\n", installer=self.ENTRY)

    def test_all_installer_entries_use_guarded_kernel_and_initrd(self):
        for bad in (self.ENTRY.replace("/install/vmlinuz", "/other/vmlinuz"),
                    self.ENTRY.replace("/install/initrd.gz", "/other/initrd.gz")):
            with self.subTest(entry=bad):
                with self.assertRaisesRegex(ValueError, "unexpected"):
                    self.check(installer=self.ENTRY, extras={"install.cfg": bad})

    def test_external_preseed_or_command_override_refused(self):
        for override in ("url=https://example.invalid/test.cfg", "preseed/url=https://example.invalid/test.cfg",
                         "preseed/early_command=true", "file=/other/preseed.cfg"):
            with self.subTest(override=override):
                bad = self.ENTRY.replace("vga=normal", "vga=normal " + override)
                with self.assertRaisesRegex(ValueError, "overrides"):
                    self.check(installer=bad)

    def test_installer_missing_initrd_refused(self):
        with self.assertRaisesRegex(ValueError, "matching|lacks"):
            self.check(installer=self.ENTRY.replace(" initrd /install/initrd.gz\n", ""))


class FreshEvidence(unittest.TestCase):
    def test_completion_marker_survives_installer_progress_on_same_console(self):
        emission = m.CI_FINISH.strip().splitlines()[-1]
        self.assertTrue(emission.startswith("printf "))
        self.assertTrue(emission.endswith(">/dev/ttyS0"))
        raw = subprocess.check_output(["sh", "-c", emission.rsplit(">", 1)[0]])
        evidence = m.StageEvidence("install")
        evidence.feed(b"Finishing installation ... 10%" + raw)
        self.assertTrue(evidence.accepted)

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
