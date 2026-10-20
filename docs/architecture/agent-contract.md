# Agent IPC Contract — Proposed

The unprivileged core communicates with the host agent over a root/locas-group controlled Unix socket. Socket permissions do not replace peer credential verification. No arbitrary shell/command RPC is permitted.

Allowed operation categories (future implementations only):
- GetHealth
- GetHardwareSnapshot
- ListManagedContainers
- StartApprovedApplication(application_id)
- StopApprovedApplication(application_id)

Every mutating request must include an authenticated principal, correlation ID, operation identifier, and server-side authorization decision. The agent validates a separate allowlist of operation types and allowed application identities. Never trust a client-provided path or Compose content without validation.

Status: specification only. There is no implemented privileged RPC or authentication as of LOS-004.
