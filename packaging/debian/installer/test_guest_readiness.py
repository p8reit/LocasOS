import importlib.util
from pathlib import Path
from unittest import mock
import json
import io
import unittest

spec = importlib.util.spec_from_file_location(
    "guest_readiness", Path(__file__).with_name("guest_readiness.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class GuestReadinessTests(unittest.TestCase):
    def test_health_marker_starts_its_own_serial_line(self):
        output = io.StringIO("debian login: ")
        output.seek(0, 2)
        m.write_marker(output, m.MARKER)
        self.assertEqual(output.getvalue().splitlines()[-1], m.MARKER)

    def test_missing_uart_uses_journal_output_without_closing_stdout(self):
        output = io.StringIO()
        with mock.patch.object(m.os, "open", side_effect=OSError()), mock.patch.object(m.sys, "stdout", output):
            with m.diagnostics_stream(Path("/dev/ttyS0")) as stream:
                stream.write("health\n")
        self.assertFalse(output.closed)
        self.assertEqual(output.getvalue(), "health\n")

    def test_uart_is_opened_without_waiting_for_carrier(self):
        with mock.patch.object(m.os, "open", side_effect=OSError()) as opened:
            with m.diagnostics_stream(Path("/dev/ttyS0")):
                pass
        self.assertEqual(opened.call_args.args[1], m.os.O_WRONLY | m.os.O_NONBLOCK | m.os.O_NOCTTY)

    def test_exact_health_json(self):
        self.assertTrue(m.healthy_payload(b'{"component":"locas-core","status":"ok"}'))
        for raw in (b'', b'{"status":"ok"}',
                    b'{"component":"locas-core","status":"degraded"}',
                    b'{"component":"other","status":"ok"}',
                    b'{"nested":{"status":"ok"},"component":"locas-core"}',
                    b'["locas-core","ok"]', b'invalid'):
            with self.subTest(raw=raw):
                self.assertFalse(m.healthy_payload(raw))

    def test_systemd_errors_fail_closed(self):
        with mock.patch.object(m.subprocess, "run", side_effect=OSError()):
            self.assertFalse(m.systemd_ready())

    def test_systemd_target_and_services_required(self):
        with mock.patch.object(m.subprocess, "run") as run:
            run.return_value.returncode = 0
            self.assertTrue(m.systemd_ready())
            seen = [call.args[0][-1] for call in run.call_args_list]
            self.assertEqual(seen, ["multi-user.target", "dbus.service", "locas-agent.service", "locas-core.service"])
            run.reset_mock()
            run.side_effect = [mock.Mock(returncode=0), mock.Mock(returncode=1)]
            self.assertFalse(m.systemd_ready())
            self.assertEqual(run.call_count, 2)

    def test_guest_health_never_accepts_unrelated_status(self):
        self.assertFalse(m.healthy_payload(json.dumps({"status": "ok", "component": "qemu"}).encode()))


if __name__ == "__main__":
    unittest.main()
