> Review notice (2026-11-02): This document includes historical plans or implementation claims that predate checkout `9f2aabe`. Consult the [current feature and phase map](../../project/README.md) for reviewed status, remaining work and guardrails. Existing release/security requirements still apply; this notice does not certify them as satisfied.

# Hardware snapshot boundary

LOS-008 provides a testable Linux host collector, not a public endpoint. Data comes from /proc/meminfo and Go runtime CPU count. GPUs remain empty pending LOS-009.

Do not publish /api/v1/hardware before identity and core-to-agent authorization are implemented. Acceptance outstanding: authenticated route and target hardware tests.
