# LOS-016: Agent telemetry (internal, read-only)

The host agent owns Linux memory/CPU and NVIDIA GPU discovery. It exposes `GET /internal/telemetry` on its existing Unix-domain socket only. It does **not** expose this endpoint through the public management API.

The handler reports a distinct `gpu_status: unavailable` when the GPU driver or probe is missing. No fabricated GPU metrics are reported. Host probe failures return HTTP 503. The endpoint makes no Docker calls and accepts no commands.

**Security gate:** A Unix socket's filesystem permissions are necessary but not sufficient for future privileged operations. Before forwarding telemetry to browser clients or adding Docker operations, implement SO_PEERCRED-based identity verification, explicit authorization and authenticated management sessions. Never proxy arbitrary URLs or paths from the client.

The endpoint's scope is diagnostic read access for processes already authorized to connect to the agent Unix socket. Container access to this socket must remain prohibited.
