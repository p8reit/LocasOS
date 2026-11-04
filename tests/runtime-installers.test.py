#!/usr/bin/env python3
"""Run production installers with fixed fake commands and a temporary filesystem.

Only fixed appliance paths are relocated in script copies. PATH contains selected
filesystem tools and our stubs, so no real package manager, Docker, service manager,
archive extraction or network command can run. This does not validate WSL, image
availability, upstream archive contents, Docker or actual application readiness.
"""
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class RuntimeInstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'root'
        self.bin = self.base / 'bin'
        self.bin.mkdir()
        for name in ('etc/systemd/system', 'usr/local/bin', 'var/run',
                     'var/lib/locas-ollama/models', 'var/lib/locas-open-webui/data',
                     'proc/net', 'proc/4242/fd', 'proc/4343/fd'):
            (self.root / name).mkdir(parents=True)
        (self.root / 'proc/4242/fd/3').symlink_to('socket:[12345]')
        (self.root / 'proc/4343/fd/3').symlink_to('socket:[12345]')
        (self.base / 'main-pid').write_text('4242\n')
        (self.base / 'owned-listener').write_text(self.socket_table())
        (self.base / 'foreign-listener').write_text(self.socket_table(inode='99999'))
        self.listener()
        (self.root / 'etc/wsl.conf').write_text('[boot]\nsystemd=true\n')
        (self.root / 'etc/systemd/system/locas-core.service').touch()
        self.model = self.root / 'var/lib/locas-ollama/models/keep'
        self.model.write_text('existing model')
        self.data = self.root / 'var/lib/locas-open-webui/data/keep'
        self.data.write_text('existing database')
        self.secret = self.root / 'var/lib/locas-open-webui/secret'
        self.secret.write_text('a' * 64)
        self.secret.chmod(0o600)
        # Deliberately exclude host install/network/service commands from PATH.
        for tool in ('grep', 'head', 'od', 'tr', 'mkdir', 'chmod', 'cat', 'rm', 'mktemp'):
            path = shutil.which(tool)
            self.assertIsNotNone(path, tool)
            (self.bin / tool).symlink_to(path)
        self.command('id', 'printf "%s\\n" "${TEST_UID:-0}"')
        self.command('sleep', ':')
        self.command('systemctl', '''printf 'systemctl %s\\n' "$*" >> "$CALL_LOG"
[[ "${FAIL_SYSTEMCTL:-}" != "$1:${!#}" ]] || exit 1
case "$1" in
  list-unit-files) echo 'locas-core.service enabled';;
  is-active) [[ "${INACTIVE_SERVICE:-}" != "${!#}" ]] || exit 3;;
  show) cat "$TEST_BASE/main-pid";;
  enable)
    if [[ "${!#}" == locas-ollama.service ]]; then
      if [[ "${FOREIGN_AFTER_START:-}" == 1 ]]; then
        cat "$TEST_BASE/foreign-listener" > "$TEST_ROOT/proc/net/tcp"
      else
        cat "$TEST_BASE/owned-listener" > "$TEST_ROOT/proc/net/tcp"
      fi
    fi;;
esac''')
        self.command('curl', '''printf 'curl %s\\n' "$*" >> "$CALL_LOG"
case "${!#}" in
  https://ollama.com/download/ollama-linux-amd64.tar.zst)
    [[ "${FAIL_DOWNLOAD:-}" != 1 ]]; exit $?;;
  http://127.0.0.1:*/api/tags) kind=OLLAMA;;
  http://127.0.0.1:59421/health) kind=WEBUI;;
  *) exit 90;;
esac
counter="$TEST_BASE/curl-$kind"
count=0
[[ ! -f "$counter" ]] || read -r count < "$counter"
count=$((count+1)); echo "$count" > "$counter"
status_name="${kind}_HTTP"; delay_name="${kind}_DELAY"
if (( count <= ${!delay_name:-0} )); then echo -n 503; else echo -n "${!status_name:-200}"; fi
if [[ "$kind" == OLLAMA && "${FOREIGN_DURING_HTTP:-}" == 1 ]]; then
  cat "$TEST_BASE/foreign-listener" > "$TEST_ROOT/proc/net/tcp"
fi
if [[ "$kind" == OLLAMA && "${PID_FLAP_DURING_HTTP:-}" == 1 ]]; then
  read -r pid < "$TEST_BASE/main-pid"
  if [[ "$pid" == 4242 ]]; then echo 4343; else echo 4242; fi > "$TEST_BASE/main-pid"
fi
[[ "${FAIL_CURL:-}" != 1 || "${FAIL_CURL_KIND:-$kind}" != "$kind" ]]''')
        self.command('tar', '''printf 'tar %s\\n' "$*" >> "$CALL_LOG"
if [[ " $* " == *' -tf '* ]]; then
  echo "${ARCHIVE_PREFIX:-}bin/ollama"
  [[ "${ARCHIVE_LAYOUT:-}" == missing-runner ]] || echo "${ARCHIVE_PREFIX:-}lib/ollama/llama-server"
else
  [[ "${!#}" == "$TEST_ROOT/usr/local" ]] || exit 91
  mkdir -p "$TEST_ROOT/usr/local/bin" "$TEST_ROOT/usr/local/lib/ollama"
  echo runtime > "$TEST_ROOT/usr/local/bin/ollama"
  echo runner > "$TEST_ROOT/usr/local/lib/ollama/llama-server"
  chmod 0755 "$TEST_ROOT/usr/local/bin/ollama" "$TEST_ROOT/usr/local/lib/ollama/llama-server"
fi''')
        self.docker = self.base / 'docker-fixture'
        self.docker.write_text('''#!/bin/bash
printf 'docker %s\\n' "$*" >> "$CALL_LOG"
[[ "$1" == --host && "$2" == "unix://$TEST_ROOT/var/run/docker.sock" ]] || exit 92
[[ -z "${DOCKER_HOST:-}${DOCKER_CONTEXT:-}${DOCKER_TLS_VERIFY:-}${DOCKER_CERT_PATH:-}" ]] || exit 93
shift 2
case "$1:$2" in
  container:inspect)
    if [[ " $* " == *'.Config.Env'* ]]; then
      [[ "${FAIL_RUNTIME_INSPECT:-}" != 1 ]] || exit 1
      echo "OLLAMA_BASE_URL=${CONTAINER_RUNTIME:-http://127.0.0.1:11434}"
      [[ "${DUPLICATE_RUNTIME:-}" != 1 ]] || echo 'OLLAMA_BASE_URL=http://127.0.0.1:11434'
    elif [[ " $* " == *' --format '* ]]; then echo "${CONTAINER_RUNNING:-true}"
    else [[ "${EXISTING_CONTAINER:-}" == 1 ]]; fi;;
  start:*) [[ "${FAIL_START:-}" != 1 ]];;
  pull:*) [[ "${FAIL_PULL:-}" != 1 ]];;
  run:*)
    while (( $# )); do
      if [[ "$1" == --env-file ]]; then
        shift
        [[ -f "$1" && ! -L "$1" ]] || exit 94
        [[ "$(cat "$1")" == "WEBUI_SECRET_KEY=$(cat "$TEST_ROOT/var/lib/locas-open-webui/secret")" ]] || exit 95
        [[ "$(/usr/bin/stat -c %a "$1")" == 600 ]] || exit 96
        echo "$1" > "$TEST_BASE/env-file-path"
      fi
      shift
    done
    [[ "${FAIL_RUN:-}" != 1 ]];;
  *) exit 97;;
esac
''')
        self.docker.chmod(0o755)
        self.command('apt-get', '''printf 'apt-get %s\\n' "$*" >> "$CALL_LOG"
[[ "${FAIL_APT:-}" != 1 ]] || exit 1
if [[ "$1" == install ]]; then
  # Debian 13 docker.io is daemon-only; --no-install-recommends omits CLI.
  if [[ " $* " == *' docker-cli '* ]]; then
    /bin/ln -s "$TEST_BASE/docker-fixture" "$TEST_BASE/bin/docker"
  fi
  printf '#!/bin/bash\\nexit 0\\n' > "$TEST_BASE/bin/dockerd"
  chmod 0755 "$TEST_BASE/bin/dockerd"
fi''')
        self.env = dict(os.environ, PATH=str(self.bin), TEST_BASE=str(self.base),
                        TEST_ROOT=str(self.root), CALL_LOG=str(self.base / 'calls'),
                        TMPDIR=str(self.base), DOCKER_HOST='tcp://unmanaged.invalid:2375',
                        DOCKER_CONTEXT='unmanaged', DOCKER_TLS_VERIFY='1', DOCKER_CERT_PATH='/unmanaged',
                        OLLAMA_HOST='127.0.0.1:11434')
        self.scripts = {}
        for app in ('ollama', 'open-webui'):
            source = (ROOT / f'packaging/wsl/install-{app}.sh').read_text()
            for path in ('/etc/wsl.conf', '/etc/systemd/system', '/usr/local',
                         '/var/lib/locas-open-webui', '/var/run/docker.sock', '/proc/'):
                source = source.replace(path, str(self.root) + path)
            script = self.base / f'install-{app}.sh'
            script.write_text(source)
            self.scripts[app] = script

    @staticmethod
    def socket_table(address='0100007F', inode='12345', port=11434):
        return ('header\n'
                f'0: {address}:{port:04X} 00000000:0000 0A 0:0 00:0 0 1000 0 {inode} 1\n')

    def listener(self, address='0100007F', inode='12345', port=11434):
        table = self.socket_table(address, inode, port) if address is not None else 'header\n'
        (self.root / 'proc/net/tcp').write_text(table)
        (self.root / 'proc/net/tcp6').write_text('header\n')

    def configure_port(self, port):
        self.env['OLLAMA_HOST'] = f'127.0.0.1:{port}'
        (self.base / 'owned-listener').write_text(self.socket_table(port=port))
        (self.base / 'foreign-listener').write_text(self.socket_table(inode='99999', port=port))
        self.listener(port=port)

    def command(self, name, body):
        path = self.bin / name
        path.write_text('#!/bin/bash\nset -e\n' + body + '\n')
        path.chmod(0o755)

    def runtime(self):
        for name in ('bin/ollama', 'lib/ollama/llama-server'):
            path = self.root / 'usr/local' / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('existing runtime')
            path.chmod(0o755)

    def docker_installed(self):
        (self.bin / 'docker').symlink_to(self.docker)
        self.command('dockerd', ':')

    def run_script(self, app, **env):
        return subprocess.run(['/bin/bash', str(self.scripts[app])],
                              env=dict(self.env, **env), text=True,
                              capture_output=True, timeout=15)

    def calls(self):
        path = self.base / 'calls'
        return path.read_text() if path.exists() else ''

    def assert_preserved(self):
        self.assertEqual(self.model.read_text(), 'existing model')
        self.assertEqual(self.data.read_text(), 'existing database')
        self.assertEqual(self.secret.read_text(), 'a' * 64)

    def assert_failure(self, result, stage):
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn(f'LOCAS_INSTALL_ERROR={stage}\n', result.stderr)
        self.assert_preserved()

    def test_fresh_ollama_waits_for_api_and_accepts_dot_archive_prefix(self):
        self.listener(None)
        result = self.run_script('ollama', ARCHIVE_PREFIX='./', OLLAMA_DELAY='2')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.base / 'curl-OLLAMA').read_text().strip(), '3')
        self.assertIn('--noproxy * --proto =http', self.calls())
        self.assert_preserved()

    def test_existing_ollama_retries_without_download_or_overwrite(self):
        self.runtime()
        result = self.run_script('ollama')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('https://ollama.com', self.calls())
        self.assertEqual((self.root / 'usr/local/bin/ollama').read_text(), 'existing runtime')
        self.assert_preserved()

    def test_invalid_runtime_host_is_rejected_without_actions(self):
        invalid = ('', '0.0.0.0:11435', 'localhost:11435', '[::1]:11435', '127.0.0.1',
                   '127.0.0.1:22', '127.0.0.1:65536', '127.0.0.1:59420',
                   '127.0.0.1:59421', '127.0.0.1:11435;touch /tmp/forbidden')
        for app in ('ollama', 'open-webui'):
            for host in invalid:
                with self.subTest(app=app, host=host):
                    result = self.run_script(app, OLLAMA_HOST=host)
                    stage = 'ollama-preflight' if app == 'ollama' else 'webui-preflight'
                    self.assert_failure(result, stage)
                    self.assertEqual(self.calls(), '')

    def test_operator_selected_port_works_without_reusing_default_listener(self):
        self.configure_port(11435)
        # The foreign default listener must neither block nor satisfy this check.
        self.listener(inode='99999', port=11434)
        result = self.run_script('ollama')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('http://127.0.0.1:11435/api/tags', self.calls())
        self.assertNotIn('http://127.0.0.1:11434/api/tags', self.calls())
        self.assertNotIn('systemctl stop', self.calls())
        self.assert_preserved()

    def test_unset_runtime_host_preserves_the_default(self):
        del self.env['OLLAMA_HOST']
        self.runtime()
        for app in ('ollama', 'open-webui'):
            with self.subTest(app=app):
                result = self.run_script(app)
                self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('http://127.0.0.1:11434/api/tags', self.calls())
        self.assertIn('-e OLLAMA_BASE_URL=http://127.0.0.1:11434', self.calls())

    def test_incomplete_existing_runtime_requires_explicit_repair(self):
        path = self.root / 'usr/local/bin/ollama'
        path.touch()
        result = self.run_script('ollama')
        self.assert_failure(result, 'ollama-preflight')
        self.assertEqual(path.read_bytes(), b'')

    def test_missing_runner_rejected_before_extraction(self):
        result = self.run_script('ollama', ARCHIVE_LAYOUT='missing-runner')
        self.assert_failure(result, 'ollama-archive')
        self.assertFalse((self.root / 'usr/local/bin/ollama').exists())

    def test_ollama_active_unit_without_api_does_not_succeed(self):
        self.runtime()
        result = self.run_script('ollama', OLLAMA_HTTP='503')
        self.assert_failure(result, 'ollama-readiness')
        self.assertEqual((self.base / 'curl-OLLAMA').read_text().strip(), '30')

    def test_ollama_rejects_redirect_and_failed_request(self):
        self.runtime()
        for http, fail in (('302', '0'), ('200', '1')):
            with self.subTest(http=http, fail=fail):
                result = self.run_script('ollama', OLLAMA_HTTP=http, FAIL_CURL=fail)
                self.assert_failure(result, 'ollama-readiness')

    def test_ollama_service_failure_is_classified(self):
        self.runtime()
        self.assert_failure(self.run_script('ollama', FAIL_SYSTEMCTL='enable:locas-ollama.service'),
                            'ollama-service')

    def test_foreign_listener_fails_before_download_or_service_mutation(self):
        self.listener(inode='99999')
        result = self.run_script('ollama')
        self.assert_failure(result, 'ollama-port-conflict')
        self.assertNotIn('curl ', self.calls())
        self.assertNotIn('systemctl enable', self.calls())
        self.assertNotIn('systemctl daemon-reload', self.calls())
        self.assertNotIn('systemctl stop', self.calls())
        self.assertFalse((self.root / 'usr/local/bin/ollama').exists())

    def test_owned_wildcard_listener_cannot_pass_loopback_contract(self):
        self.listener(address='00000000')
        self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')
        self.assertNotIn('curl ', self.calls())

    def test_ipv6_wildcard_listener_is_not_treated_as_a_free_port(self):
        self.listener(None)
        (self.root / 'proc/net/tcp6').write_text(self.socket_table(address='0' * 32, inode='99999'))
        self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')

    def test_unverifiable_socket_owner_fails_closed(self):
        shutil.rmtree(self.root / 'proc/4242')
        self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')

    def test_missing_or_malformed_socket_table_fails_closed(self):
        table = self.root / 'proc/net/tcp'
        table.unlink()
        self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')
        table.write_text('header\nmalformed kernel row\n')
        self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')
        table.write_text(self.socket_table(address='not-an-address'))
        self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')

    def test_main_pid_must_be_a_positive_decimal_pid(self):
        for pid in ('0', '../4242', '-1', 'not-a-pid'):
            with self.subTest(pid=pid):
                (self.base / 'main-pid').write_text(pid + '\n')
                self.assert_failure(self.run_script('ollama'), 'ollama-port-conflict')

    def test_foreign_listener_after_start_cannot_pass_a_healthy_api(self):
        self.listener(None)
        result = self.run_script('ollama', FOREIGN_AFTER_START='1')
        self.assert_failure(result, 'ollama-readiness')
        self.assertNotIn('http://127.0.0.1:11434/api/tags', self.calls())
        self.assertNotIn('systemctl stop', self.calls())

    def test_owner_rechecked_after_successful_http(self):
        self.runtime()
        result = self.run_script('ollama', FOREIGN_DURING_HTTP='1')
        self.assert_failure(result, 'ollama-readiness')
        self.assertEqual((self.base / 'curl-OLLAMA').read_text().strip(), '1')

    def test_main_pid_must_remain_stable_during_http(self):
        self.runtime()
        result = self.run_script('ollama', PID_FLAP_DURING_HTTP='1')
        self.assert_failure(result, 'ollama-readiness')

    def test_real_linux_proc_probe_proves_fd_ownership(self):
        source = (ROOT / 'packaging/wsl/install-ollama.sh').read_text()
        probe = source.split("<<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
        # Bind an ephemeral test port; production accepts only validated config.
        with socket.socket(socket.AF_INET) as listener:
            listener.bind(('127.0.0.1', 0))
            listener.listen()
            port = listener.getsockname()[1]
            def run_probe(pid):
                result = subprocess.run([sys.executable, '-I', '-', str(pid), str(port)], input=probe,
                                        text=True, capture_output=True, timeout=5)
                self.assertEqual(result.stdout, '')
                self.assertEqual(result.stderr, '')
                return result
            self.assertEqual(run_probe(os.getpid()).returncode, 0)
            # Namespace-wide /proc/net visibility alone must not prove ownership.
            self.assertEqual(run_probe(0).returncode, 3)
        self.assertEqual(run_probe(os.getpid()).returncode, 2)

    def test_fresh_webui_installs_separate_cli_and_waits_for_health(self):
        result = self.run_script('open-webui', WEBUI_DELAY='2')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('apt-get install -y --no-install-recommends docker.io docker-cli', self.calls())
        self.assertEqual((self.base / 'curl-WEBUI').read_text().strip(), '3')
        self.assertIn('--env-file ', self.calls())
        self.assertNotIn('a' * 64, self.calls() + result.stdout + result.stderr)
        self.assertFalse(Path((self.base / 'env-file-path').read_text().strip()).exists())
        self.assert_preserved()

    def test_fresh_webui_uses_explicitly_configured_runtime_port(self):
        self.configure_port(11435)
        result = self.run_script('open-webui')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('-e OLLAMA_BASE_URL=http://127.0.0.1:11435', self.calls())
        self.assertIn('http://127.0.0.1:11435/api/tags', self.calls())
        self.assert_preserved()

    def test_existing_webui_failed_start_is_an_error(self):
        self.docker_installed()
        result = self.run_script('open-webui', EXISTING_CONTAINER='1', FAIL_START='1')
        self.assert_failure(result, 'webui-container-start')
        self.assertNotIn('docker pull', self.calls())
        self.assertNotIn('docker run', self.calls())

    def test_existing_webui_preserves_container_and_configuration(self):
        self.docker_installed()
        result = self.run_script('open-webui', EXISTING_CONTAINER='1')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('docker pull', self.calls())
        self.assertNotIn('docker run', self.calls())
        self.assert_preserved()

    def test_existing_webui_matching_operator_port_can_start(self):
        self.configure_port(11435)
        self.docker_installed()
        result = self.run_script('open-webui', EXISTING_CONTAINER='1',
                                 CONTAINER_RUNTIME='http://127.0.0.1:11435')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('start locas-open-webui', self.calls())
        self.assertNotIn('pull ', self.calls())
        self.assertNotIn('run -d ', self.calls())
        self.assert_preserved()

    def test_existing_webui_endpoint_mismatch_requires_explicit_reconciliation(self):
        self.configure_port(11435)
        self.docker_installed()
        result = self.run_script('open-webui', EXISTING_CONTAINER='1')
        self.assert_failure(result, 'webui-runtime-config')
        self.assertNotIn('start locas-open-webui', self.calls())
        self.assertNotIn('pull ', self.calls())
        self.assertNotIn('run -d ', self.calls())

    def test_existing_webui_ambiguous_or_unreadable_endpoint_fails_closed(self):
        self.docker_installed()
        for env in ({'DUPLICATE_RUNTIME': '1'}, {'FAIL_RUNTIME_INSPECT': '1'}):
            with self.subTest(env=env):
                result = self.run_script('open-webui', EXISTING_CONTAINER='1', **env)
                self.assert_failure(result, 'webui-runtime-config')
                self.assertNotIn('start locas-open-webui', self.calls())

    def test_webui_rejects_redirect_error_and_failed_health_request(self):
        self.docker_installed()
        for http, fail in (('302', '0'), ('503', '0'), ('200', '1')):
            with self.subTest(http=http, fail=fail):
                result = self.run_script('open-webui', EXISTING_CONTAINER='1', WEBUI_HTTP=http,
                                         FAIL_CURL=fail, FAIL_CURL_KIND='WEBUI')
                self.assert_failure(result, 'webui-readiness')

    def test_webui_stopped_container_cannot_pass_unrelated_http_health(self):
        self.docker_installed()
        result = self.run_script('open-webui', EXISTING_CONTAINER='1', CONTAINER_RUNNING='false')
        self.assert_failure(result, 'webui-readiness')
        self.assertNotIn('http://127.0.0.1:59421/health', self.calls())

    def test_webui_requires_ready_ollama_before_package_or_container_changes(self):
        result = self.run_script('open-webui', OLLAMA_HTTP='503')
        self.assert_failure(result, 'webui-preflight')
        self.assertNotIn('apt-get ', self.calls())
        self.assertNotIn('docker ', self.calls())

    def test_webui_rejects_foreign_ollama_even_when_unit_and_http_are_healthy(self):
        self.listener(inode='99999')
        result = self.run_script('open-webui')
        self.assert_failure(result, 'webui-preflight')
        self.assertNotIn('apt-get ', self.calls())
        self.assertNotIn('docker ', self.calls())
        self.assertNotIn('curl ', self.calls())

    def test_webui_run_failure_cleans_secret_file(self):
        self.docker_installed()
        result = self.run_script('open-webui', FAIL_RUN='1')
        self.assert_failure(result, 'webui-container-start')
        self.assertFalse(Path((self.base / 'env-file-path').read_text().strip()).exists())
        self.assertNotIn('a' * 64, self.calls() + result.stdout + result.stderr)

    def test_unmanaged_docker_socket_is_rejected(self):
        sock = socket.socket(socket.AF_UNIX)
        self.addCleanup(sock.close)
        sock.bind(str(self.root / 'var/run/docker.sock'))
        result = self.run_script('open-webui', INACTIVE_SERVICE='docker.service')
        self.assert_failure(result, 'webui-preflight')
        self.assertNotIn('apt-get ', self.calls())
        self.assertNotIn('docker ', self.calls())

    def test_preflight_and_package_failure_are_classified(self):
        self.assert_failure(self.run_script('open-webui', TEST_UID='1000'), 'webui-preflight')
        self.assert_failure(self.run_script('open-webui', FAIL_APT='1'), 'webui-docker-install')


if __name__ == '__main__':
    unittest.main()
