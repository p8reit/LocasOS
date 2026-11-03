"""Exercise the production upgrade script against a temporary filesystem.

Only its fixed root constant is substituted. Commands that operate on services,
identity and health are mocked; tar, checksums, install and restore are real.
This is failure-injection coverage, not Windows/WSL validation.
"""
import fcntl
import hashlib
import io
import os
from pathlib import Path
import shlex
import subprocess
import tarfile
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1] / 'packaging/wsl/upgrade-locas.sh'
CORE = 'opt/locas/bin/locas-core'
AGENT = 'opt/locas/bin/locas-agent'
SCRIPT = 'usr/local/sbin/locas-upgrade'
UNIT = 'etc/systemd/system/locas-core.service'
NEW = 'etc/systemd/system/locas-update-worker.path'

class UpgradeTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'root'
        for name in ['run', 'var/lib', 'opt/locas/bin', 'etc/systemd/system', 'usr/local/sbin']:
            (self.root / name).mkdir(parents=True, exist_ok=True)
        (self.root/'etc/os-release').write_text('ID=debian\nVERSION_ID=13\n')
        self.original = {CORE: b'old-core', AGENT: b'old-agent', SCRIPT: b'old-upgrade', UNIT: b'old-unit'}
        for name, data in self.original.items():
            p = self.root / name
            p.write_bytes(data)
            p.chmod(0o755 if name.startswith(('opt/', 'usr/')) else 0o640)
        self.data = self.root / 'var/lib/models/model.bin'
        self.data.parent.mkdir(parents=True)
        self.data.write_bytes(b'valuable-model')
        self.bin = self.base / 'bin'
        self.bin.mkdir()
        self.command('id', 'echo 0')
        self.command('install', '''args=()
while (( $# )); do
  case "$1" in -o|-g) shift 2;; *) args+=("$1"); shift;; esac
done
last="${args[-1]}"
if [[ "${FAIL_INSTALL:-}" == 1 && "$last" == */usr/local/sbin/locas-upgrade && ! -f "$FAIL_MARKER" ]]; then
  touch "$FAIL_MARKER"; exit 1
fi
exec /usr/bin/install "${args[@]}"''')
        self.command('systemctl', '''echo "$*" >> "$CALL_LOG"
case "$1" in
  is-enabled) echo disabled; exit 1;;
  is-active) if [ "$2" = --quiet ]; then exit 0; else echo inactive; exit 3; fi;;
  restart) if [ "${FAIL_RESTART:-}" = 1 ] && [ ! -f "$FAIL_MARKER" ]; then touch "$FAIL_MARKER"; exit 1; fi;;
esac
exit 0''')
        self.command('curl', '[ "${FAIL_HEALTH:-}" != 1 ]')
        self.command('sleep', ':')
        self.command('journalctl', ':')
        self.script = self.base / 'upgrade.sh'
        self.script.write_text(SOURCE.read_text().replace('local root=/', 'local root=' + shlex.quote(str(self.root)), 1))
        self.env = dict(os.environ, PATH=str(self.bin) + ':' + os.environ['PATH'],
                        CALL_LOG=str(self.base/'calls'), FAIL_MARKER=str(self.base/'failed'))

    def command(self, name, body):
        p = self.bin / name
        p.write_text('#!/bin/bash\n' + body + '\n')
        p.chmod(0o755)

    def bundle(self, extras=None, link=None):
        self.archive = self.base / 'update.tar'
        self.payload = {'opt/locas/bin/locas-os-base': b'debian:13', CORE: b'new-core', AGENT: b'new-agent', SCRIPT: b'new-upgrade', UNIT: b'new-unit', NEW: b'new-worker'}
        self.payload.update(extras or {})
        with tarfile.open(self.archive, 'w') as archive:
            for name, data in self.payload.items():
                info = tarfile.TarInfo(name)
                info.size = len(data)
                info.mode = 0o755
                archive.addfile(info, io.BytesIO(data))
            if link:
                info = tarfile.TarInfo('usr/local/sbin/locas-app-worker')
                info.type = tarfile.SYMTYPE
                info.linkname = link
                archive.addfile(info)
        Path(str(self.archive)+'.sha256').write_text(hashlib.sha256(self.archive.read_bytes()).hexdigest()+'  update.tar\n')

    def run_upgrade(self, **env):
        return subprocess.run(['bash', str(self.script), str(self.archive)], env=dict(self.env, **env),
                              text=True, capture_output=True, timeout=15)

    def assert_restored(self):
        for name, data in self.original.items():
            self.assertEqual((self.root/name).read_bytes(), data, name)
        self.assertEqual((self.root/UNIT).stat().st_mode & 0o777, 0o640)
        self.assertFalse((self.root/NEW).exists())
        self.assertEqual(self.data.read_bytes(), b'valuable-model')

    def test_success_installs_files_and_activates_both_workers(self):
        self.bundle()
        result = self.run_upgrade()
        self.assertEqual(result.returncode, 0, result.stderr)
        for name, data in self.payload.items():
            self.assertEqual((self.root/name).read_bytes(), data)
        self.assertIn('enable --now locas-app-worker.path', (self.base/'calls').read_text())
        self.assertIn('enable --now locas-update-worker.path', (self.base/'calls').read_text())
        self.assertEqual(self.data.read_bytes(), b'valuable-model')

    def test_restart_failure_restores_scripts_units_and_removes_new_files(self):
        self.bundle()
        result = self.run_upgrade(FAIL_RESTART='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('restoring all replaced', result.stderr)
        self.assert_restored()

    def test_health_failure_rolls_back(self):
        self.bundle()
        result = self.run_upgrade(FAIL_HEALTH='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('restoring all replaced', result.stderr)
        self.assert_restored()

    def test_rejects_data_overwrite(self):
        self.bundle({'var/lib/models/model.bin': b'corrupt'})
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assert_restored()

    def test_rejects_parent_traversal(self):
        self.bundle({'../escape': b'bad'})
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.base/'escape').exists())
        self.assert_restored()

    def test_rejects_archive_links(self):
        self.bundle(link='/etc/passwd')
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assert_restored()

    def test_rejects_destination_symlink(self):
        self.bundle()
        (self.root/NEW).symlink_to(self.data)
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.data.read_bytes(), b'valuable-model')
        self.assertEqual((self.root/CORE).read_bytes(), b'old-core')

    def test_worker_does_not_require_writable_run(self):
        self.bundle()
        (self.root/'run').chmod(0o555)
        result = self.run_upgrade()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.root/'var/lib/locas-upgrades/upgrade.lock').exists())

    def test_upgrade_lock_blocks_competing_installer(self):
        self.bundle()
        directory=self.root/'var/lib/locas-upgrades'
        directory.mkdir()
        with (directory/'upgrade.lock').open('w') as lock:
            fcntl.flock(lock,fcntl.LOCK_EX | fcntl.LOCK_NB)
            result=self.run_upgrade()
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Another LOCAS upgrade is running',result.stderr)
        self.assert_restored()

    def test_partial_install_failure_rolls_back(self):
        self.bundle()
        result = self.run_upgrade(FAIL_INSTALL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('restoring all replaced', result.stderr)
        self.assert_restored()

    def test_cross_distribution_upgrade_rejected(self):
        self.bundle({'opt/locas/bin/locas-os-base': b'ubuntu:24.04'})
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Blocked cross-distribution', result.stderr)
        self.assert_restored()

    def test_checksum_must_name_the_archive(self):
        self.bundle()
        other = self.base/'other.tar'
        other.write_bytes(b'unrelated')
        Path(str(self.archive)+'.sha256').write_text(hashlib.sha256(other.read_bytes()).hexdigest()+'  other.tar\n')
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Checksum names another file', result.stderr)
        self.assert_restored()

    def test_checksum_failure_leaves_installation_unchanged(self):
        self.bundle()
        self.archive.write_bytes(b'corrupted')
        result = self.run_upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assert_restored()

if __name__ == '__main__':
    unittest.main()
