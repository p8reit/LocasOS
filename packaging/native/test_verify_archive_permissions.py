"""Native payload archive permissions must allow unprivileged service startup."""
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

module_path = Path(__file__).with_name("verify-archive.py")
spec = importlib.util.spec_from_file_location("verify_archive", module_path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def make_archive(path, bad_dir=None, bad_executable=None):
    with tarfile.open(path, "w") as archive:
        for name in sorted(module.ALLOWED_DIRS - {""}, key=lambda p: (p.count("/"), p)):
            item = tarfile.TarInfo(name)
            item.type = tarfile.DIRTYPE
            item.mode = 0o750 if name == bad_dir else 0o755
            archive.addfile(item)
        for name in sorted(module.ALLOWED_FILES):
            item = tarfile.TarInfo(name)
            item.mode = 0o644 if "/systemd/" in name else 0o755
            if name == bad_executable:
                item.mode = 0o750
            item.size = 1
            archive.addfile(item, io.BytesIO(b"x"))


class NativePayloadPermissionTests(unittest.TestCase):
    def check(self, *, bad_dir=None, bad_executable=None):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "payload.tar"
            make_archive(path, bad_dir, bad_executable)
            module.verify(str(path))

    def test_valid_unprivileged_executable_paths(self):
        self.check()

    def test_restrictive_opt_parent_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "inaccessible"):
            self.check(bad_dir="opt/locas")

    def test_restrictive_usr_bin_parent_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "inaccessible"):
            self.check(bad_dir="usr/local")

    def test_non_executable_agent_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "non-executable"):
            self.check(bad_executable="opt/locas/bin/locas-agent")


if __name__ == "__main__":
    unittest.main()
