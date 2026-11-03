> Review notice (2026-11-02): This document includes historical plans or implementation claims that predate checkout `9f2aabe`. Consult the [current feature and phase map](../project/README.md) for reviewed status, remaining work and guardrails. Existing release/security requirements still apply; this notice does not certify them as satisfied.

# Sprint 0 — Project Foundation

Dates: October 20–26, 2026 (planning target).

Goal: Freeze MVP boundaries and create validated implementation contracts before coding.

Tasks: LOS-001 architecture and release gates; LOS-002 Go service skeletons; LOS-003 CI; LOS-004 API and config contracts; LOS-005 read-only host preflight; LOS-006 React mock UI; LOS-007 threat model.

Dependencies: LOS-001 precedes 002/003/004/005; LOS-004 precedes 006/007.

Exit: approved ADR, contract tests, minimal build, documented privilege boundary, passing checks. No self-merging agent PRs.
