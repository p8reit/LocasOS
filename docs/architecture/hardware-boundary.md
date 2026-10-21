# Hardware snapshot boundary

LOS-008 provides a testable Linux host collector, not a public endpoint. Data comes from /proc/meminfo and Go runtime CPU count. GPUs remain empty pending LOS-009.

Do not publish /api/v1/hardware before identity and core-to-agent authorization are implemented. Acceptance outstanding: authenticated route and target hardware tests.
