# LOCAS OS v0.1 Threat Model — Draft

## Trust boundaries
1. LAN browser -> HTTPS management API: untrusted until authenticated, authorized and CSRF checked.
2. Unprivileged locas-core -> privileged locas-agent: restricted Unix socket plus SO_PEERCRED / explicit authorization and typed RPC validation.
3. Agent -> Docker daemon: equivalent to host-root authority. All Docker operations restricted by policy and recorded.
4. Docker workload -> LAN / host / models: untrusted, potentially executing malicious AI-generated commands.
5. Model files and third-party container images: untrusted supply-chain artifacts.

## Threats and minimum controls

| Threat | Mitigation | Test evidence required |
|---|---|---|
| LAN session theft or CSRF | HTTPS, secure sessions, Origin/CSRF validation | Cross-origin mutating request denied |
| Unauthorized Unix IPC | Socket permissions + peer credential verification | Different UID cannot issue operations |
| Docker socket breakout | Core never mounts or receives Docker socket | Search build configs and runtime inspect |
| Malicious Compose imports | Schema allowlist; deny privileged, host namespace, host mounts and sensitive device paths | Negative tests for every denied field |
| Agent command injection | No arbitrary shell, typed operation IDs and validated parameters | Fuzz structured requests |
| Host secrets exfiltration | Secrets outside workload mounts; egress restrictions | Containers cannot read agent or database |
| Supply chain compromise | Pin dependencies/images; validate signatures or verified checksums where supported | SBOM and vulnerability scan |
| GPU denial of service | Cooperative admission and bounded job timeout | Over-capacity managed request queues |

## Security decisions pending
- Certificate provisioning, password reset and first-boot identity enrollment.
- Exact agent socket ownership and systemd sandbox settings.
- Docker network enforcement and offline mode.
- Allowed application import policy and catalog trust/signature process.
- Backup encryption and retention.

## Release gate
No privileged management operation may ship until peer authorization, negative permission tests, audit event output and rollback documentation are in place.

This document is an initial threat assessment, not a completed security audit.
