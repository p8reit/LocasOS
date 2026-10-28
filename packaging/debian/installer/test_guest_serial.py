import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location(
    "guest_serial", Path(__file__).with_name("guest_serial.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class StageEvidenceTests(unittest.TestCase):
    def test_exact_health_marker_from_new_boot(self):
        s = m.StageEvidence("live")
        self.assertFalse(s.feed(b"kernel booting\nLOCASOS_GUEST_"))
        self.assertTrue(s.feed(b"HEALTH_OK\r\n"))
        self.assertTrue(s.accepted)

    def test_partial_or_embedded_marker_is_not_proof(self):
        s = m.StageEvidence("live")
        s.feed(b"printed LOCASOS_GUEST_HEALTH_OK on command line\n")
        self.assertFalse(s.accepted)
        s.feed(b"LOCASOS_GUEST_HEALTH_OK_EXTRA\n")
        self.assertFalse(s.accepted)

    def test_installer_marker_not_sufficient_for_disk_boot(self):
        s = m.StageEvidence("installed")
        s.feed(b"LOCASOS_VM_INSTALL_COMPLETE\nLOCASOS_GUEST_HEALTH_OK\n")
        self.assertFalse(s.accepted)
        self.assertTrue(s.feed(b"LOCASOS_INSTALLED_BOOT_OK\n"))

    def test_success_cannot_override_later_failure(self):
        s = m.StageEvidence("installed")
        self.assertTrue(s.feed(b"LOCASOS_INSTALLED_BOOT_OK\n"))
        with self.assertRaises(m.SerialFailure):
            s.feed(b"LOCASOS_GUEST_HEALTH_FAILED\n")
        self.assertFalse(s.accepted)

    def test_explicit_guest_failure_fails_closed(self):
        for event in m.FAILURES:
            with self.subTest(event=event):
                s = m.StageEvidence("install")
                with self.assertRaises(m.SerialFailure):
                    s.feed(event.encode() + b": invalid scratch disk\n")
                self.assertFalse(s.accepted)

    def test_ansi_decorated_marker_accepted_only_as_line(self):
        s = m.StageEvidence("live")
        self.assertTrue(s.feed(b"\x1b[32mLOCASOS_GUEST_HEALTH_OK\x1b[0m\n"))

    def test_separate_boots_cannot_share_evidence(self):
        stage1 = m.StageEvidence("live")
        self.assertTrue(stage1.feed(b"LOCASOS_GUEST_HEALTH_OK\n"))
        stage2 = m.StageEvidence("installed")
        self.assertFalse(stage2.accepted)

    def test_unrecognized_stage_rejected(self):
        with self.assertRaises(ValueError):
            m.StageEvidence("process-still-running")


if __name__ == "__main__":
    unittest.main()
