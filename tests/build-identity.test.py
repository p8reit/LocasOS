#!/usr/bin/env python3
"""Build provenance tests use disposable repositories, never real release tags."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
HELPER = ROOT / 'scripts/build-identity.sh'


class BuildIdentityTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name) / 'repo'
        self.repo.mkdir()
        self.git('init', '-q')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'user.name', 'Build fixture')
        (self.repo / 'source').write_text('one\n')
        self.git('add', 'source')
        self.git('commit', '-qm', 'fixture')
        self.commit = self.git('rev-parse', 'HEAD').strip()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], text=True)

    def resolve(self, tag='', repo=None):
        return subprocess.run(
            ['bash', '-euc', 'source "$1"; locas_resolve_build_identity "$2" "$3"; '
             'printf "%s %s" "$locas_build_version" "$locas_build_commit"',
             'fixture', str(HELPER), str(repo or self.repo), tag],
            text=True, capture_output=True)

    def test_development_never_infers_tag(self):
        self.git('tag', 'v1.2.3')
        result = self.resolve()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, f'dev-{self.commit[:12]} {self.commit}')

    def test_official_matching_lightweight_and_annotated_tags(self):
        self.git('tag', 'v1.2.3')
        self.git('tag', '-a', 'v1.2.4', '-m', 'fixture')
        for tag in ('v1.2.3', 'v1.2.4'):
            with self.subTest(tag=tag):
                result = self.resolve(tag)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, f'{tag} {self.commit}')

    def test_malformed_missing_and_mismatched_tags(self):
        self.git('tag', 'v1.2.3')
        (self.repo / 'source').write_text('two\n')
        self.git('commit', '-qam', 'next')
        for tag in ('v1.2.3', 'v9.9.9', '1.2.3', 'v1.2', 'v1.2.3-beta', '--help'):
            with self.subTest(tag=tag):
                self.assertNotEqual(self.resolve(tag).returncode, 0)

    def test_release_components_match_amd64_go_int_range(self):
        maximum = '9223372036854775807'
        padded = '0' * 10 + maximum
        accepted = (f'v{maximum}.0.0', f'v0.{maximum}.0', f'v0.0.{maximum}',
                    f'v{padded}.000000008.000000000000000000000')
        rejected = ('v9223372036854775808.0.0', 'v0.9223372036854775808.0',
                    'v0.0.9223372036854775808', 'v' + '9' * 50 + '.0.0',
                    'v' + '0' * 20 + '9223372036854775808.0.0')
        for tag in (*accepted, *rejected):
            self.git('tag', tag)
            result = self.resolve(tag)
            with self.subTest(tag=tag):
                if tag in accepted:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout, f'{tag} {self.commit}')
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('Go int range', result.stderr)

    def test_release_tag_fits_installed_version_record(self):
        # 63-byte tags plus newline fit the independent root reader's 64 bytes.
        for zeros, accepted in ((57, True), (58, False), (100, False)):
            tag = 'v' + '0' * zeros + '1.0.0'
            self.git('tag', tag)
            result = self.resolve(tag)
            with self.subTest(length=len(tag)):
                if accepted:
                    self.assertEqual(len(tag), 63)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout, f'{tag} {self.commit}')
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('63-byte', result.stderr)

    def test_tracked_and_untracked_dirty_release_refused(self):
        self.git('tag', 'v1.2.3')
        for name in ('source', 'untracked'):
            with self.subTest(name=name):
                path = self.repo / name
                path.write_text('dirty\n')
                self.assertNotEqual(self.resolve('v1.2.3').returncode, 0)
                self.assertEqual(self.resolve().returncode, 0)
                if name == 'source':
                    self.git('checkout', '--', name)
                else:
                    path.unlink()

    def test_no_git_development_fallback_and_release_refusal(self):
        plain = Path(self.tmp.name) / 'plain'
        plain.mkdir()
        result = self.resolve(repo=plain)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'dev unknown')
        self.assertNotEqual(self.resolve('v1.2.3', repo=plain).returncode, 0)

    def test_builders_select_identity_before_build_and_stamp_both_binaries(self):
        for builder in ('packaging/wsl/build-rootfs.sh', 'packaging/native/build-payload.sh'):
            with self.subTest(builder=builder):
                target = self.repo / builder
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy(ROOT / builder, target)
                helper = self.repo / 'scripts/build-identity.sh'
                helper.parent.mkdir(exist_ok=True)
                shutil.copy(HELPER, helper)
                fakebin = Path(self.tmp.name) / 'fakebin'
                fakebin.mkdir(exist_ok=True)
                log = Path(self.tmp.name) / 'go-arguments'
                fakego = fakebin / 'go'
                fakego.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$BUILD_TEST_LOG"\nexit 37\n')
                fakego.chmod(0o755)
                env = dict(os.environ, PATH=str(fakebin) + ':' + os.environ['PATH'],
                           BUILD_TEST_LOG=str(log), LOCAS_BUILD_VERSION='v999.0.0')
                result = subprocess.run(['bash', str(target)], env=env, capture_output=True, text=True)
                if '/native/' in builder:
                    self.assertEqual(result.returncode, 37, result.stderr)
                    flags = log.read_text()
                    self.assertIn(f'main.buildVersion=dev-{self.commit[:12]}', flags)
                    self.assertIn(f'main.buildCommit={self.commit}', flags)
                    self.assertNotIn('v999.0.0', flags)
                else:
                    self.assertNotEqual(result.returncode, 0)
                    # Explicit official tag is validated before invoking the compiler.
                    self.assertIn('Release tag does not resolve', result.stderr)
                text = target.read_text()
                for binary in ('locas-core', 'locas-updater'):
                    line = next(line for line in text.splitlines() if f'-o build/{binary} ' in line)
                    self.assertIn('main.buildVersion=${locas_build_version}', line)
                    self.assertIn('main.buildCommit=${locas_build_commit}', line)
                    self.assertIn('main.trustedPublicKeyBase64=', line)

    def test_actual_core_and_updater_binary_metadata(self):
        go = os.environ.get('LOCAS_TEST_GO')
        scratch_go = Path('/workspace/work/go-toolchain/go/bin/go')
        if not go and scratch_go.is_file():
            go = str(scratch_go)
        if not go:
            go = shutil.which('go')
        if not go:
            self.skipTest('Go toolchain unavailable for actual binary metadata test')
        self.git('tag', 'v1.2.3')
        env = dict(os.environ, CGO_ENABLED='0', GOPROXY='off', GOTOOLCHAIN='local')
        for tag in ('', 'v1.2.3'):
            identity = self.resolve(tag)
            self.assertEqual(identity.returncode, 0, identity.stderr)
            version, commit = identity.stdout.split()
            for command in ('locas-core', 'locas-updater'):
                with self.subTest(tag=tag, command=command):
                    target = Path(self.tmp.name) / command
                    build = subprocess.run(
                        [go, 'build', '-trimpath', '-ldflags',
                         f'-X main.buildVersion={version} -X main.buildCommit={commit}',
                         '-o', str(target), './cmd/' + command],
                        cwd=ROOT, env=env, text=True, capture_output=True)
                    self.assertEqual(build.returncode, 0, build.stderr)
                    result = subprocess.run([str(target), '--version'], text=True,
                                            capture_output=True, timeout=10)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(json.loads(result.stdout),
                                     {'version': version, 'commit': commit, 'release': tag})


if __name__ == '__main__':
    unittest.main()
