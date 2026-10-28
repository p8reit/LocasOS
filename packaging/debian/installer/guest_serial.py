#!/usr/bin/env python3
"""Strict stage-local serial evidence parser for isolated QEMU acceptance runs.

This parser never considers QEMU uptime, a substring, or another boot's logs
as evidence. Each VM process must provide an independent fresh parser.
"""
import re

ANSI = re.compile(r"\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))")
STAGE_MARKERS = {
    "live": "LOCASOS_GUEST_HEALTH_OK",
    "install": "LOCASOS_VM_INSTALL_COMPLETE",
    "installed": "LOCASOS_INSTALLED_BOOT_OK",
}
FAILURES = ("LOCASOS_GUEST_HEALTH_FAILED", "LOCASOS_VM_INSTALL_FAIL",
            "LOCASOS_VM_INSTALL_REFUSED")


class SerialFailure(RuntimeError):
    pass


class StageEvidence:
    """Consume only bytes from one live QEMU subprocess, not accumulated logs."""
    def __init__(self, stage):
        if stage not in STAGE_MARKERS:
            raise ValueError("unknown VM acceptance stage")
        self.stage = stage
        self.expected = STAGE_MARKERS[stage]
        self._buffer = ""
        self._found = False
        self._failed = False

    def feed(self, chunk):
        if self._failed:
            raise SerialFailure("guest already reported a failed stage")
        if isinstance(chunk, bytes):
            chunk = chunk.decode("utf-8", errors="replace")
        self._buffer += chunk.replace("\r\n", "\n").replace("\r", "\n")
        while "\n" in self._buffer:
            line, self._buffer = self._buffer.split("\n", 1)
            line = ANSI.sub("", line).strip()
            if any(line == word or line.startswith(word + ":") for word in FAILURES):
                self._failed = True
                self._found = False
                raise SerialFailure(f"guest stage failed: {line[:250]}")
            if line == self.expected:
                self._found = True
        if len(self._buffer) > 65536:
            self._buffer = self._buffer[-65536:]
        return self._found

    @property
    def accepted(self):
        return self._found and not self._failed
