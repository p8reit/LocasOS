#!/usr/bin/env python3
"""Negative tests for privileged archive ingestion."""
import io
import os
import tarfile
import tempfile
import unittest
from pathlib import Path
from importlib.machinery import SourceFileLoader

module = SourceFileLoader("verify_archive", str(Path(__file__).with_name("verify-archive.py"))).load_module()
class ArchiveTests(unittest.TestCase):
    def make_tar(self, malicious=None, skip=None):
        fd, path = tempfile.mkstemp(suffix=".tar")
        os.close(fd)
        self.addCleanup(lambda: os.unlink(path))
        with tarfile.open(path, "w") as tf:
            for name in sorted(module.ALLOWED_FILES - set(skip or ())):
                item = tarfile.TarInfo(name)
                item.size = 1
                tf.addfile(item, io.BytesIO(b"x"))
            if malicious is not None:
                tf.addfile(malicious, io.BytesIO(b"x") if malicious.isfile() and malicious.size else None)
        return path

    def test_minimal_valid_archive(self):
        module.verify(self.make_tar())

    def test_attributed_worker_and_audit_verifier_are_required(self):
        for name in ('opt/locas/bin/locas-app-worker', 'opt/locas/bin/locas-audit'):
            with self.subTest(name=name):
                self.assertIn(name, module.ALLOWED_FILES)
                with self.assertRaisesRegex(ValueError, 'missing files'):
                    module.verify(self.make_tar(skip=(name,)))

    def test_audit_journal_paths_are_never_payload(self):
        for name in ('var/lib/locas-audit/app-worker/events.jsonl',
                     'var/lib/locas-agent-audit/events.jsonl'):
            with self.subTest(name=name):
                item = tarfile.TarInfo(name)
                item.size = 1
                with self.assertRaises(ValueError):
                    module.verify(self.make_tar(item))

    def test_traversal_rejected(self):
        item = tarfile.TarInfo("../etc/passwd")
        item.size = 1
        with self.assertRaises(ValueError):
            module.verify(self.make_tar(item))

    def test_symlink_rejected(self):
        item = tarfile.TarInfo("opt/locas/bin/escape")
        item.type = tarfile.SYMTYPE
        item.linkname = "/etc/shadow"
        with self.assertRaises(ValueError):
            module.verify(self.make_tar(item))

    def test_device_rejected(self):
        item = tarfile.TarInfo("opt/locas/bin/device")
        item.type = tarfile.CHRTYPE
        with self.assertRaises(ValueError):
            module.verify(self.make_tar(item))

    def test_unexpected_file_rejected(self):
        item = tarfile.TarInfo("etc/cron.d/backdoor")
        item.size = 1
        with self.assertRaises(ValueError):
            module.verify(self.make_tar(item))

    def test_duplicate_rejected(self):
        item = tarfile.TarInfo("opt/locas/bin/locas-core")
        item.size = 1
        with self.assertRaises(ValueError):
            module.verify(self.make_tar(item))

if __name__ == "__main__":
    unittest.main()
