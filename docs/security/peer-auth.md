# LOS-017 — Unix peer authentication

Implement Linux `SO_PEERCRED` on the **accepted connection**, never on the request-supplied identity. `internal/peercred` exposes a typed primitive and tests.

## Remaining integration gate
The host agent must wrap `net.Listener.Accept` and reject unauthorized peer UIDs **before any HTTP requests are handled**. A systemd-owned socket should permit only the dedicated `locas-core` service identity. Reject root and arbitrary group members unless explicitly part of a tightly reviewed policy.

Do not expose read-only telemetry through the public API until the listener wrapper and negative integration tests are complete.

No Docker mutations, shell commands or untrusted paths are permitted as RPCs.
