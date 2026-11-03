# LOCAS OS phases

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

Phase numbers preserve names found in repository history/docs; they are not completion percentages. Phases 3 and 5 have no independent definition in the reviewed docs, so no scope is invented for them. Phase 4 lifecycle work is grouped under P02.

| Phase | Status | Feature dependencies |
|---|---|---|
| [P00: Foundation / Sprint 0](phases/P00.md) | Source foundation present; approval evidence unresolved | F01, F02, F04, F10 |
| [P01: Integration and secure telemetry](phases/P01.md) | Implemented preview | F02, F03, F04 |
| [P02: WSL appliance and lifecycle](phases/P02.md) | Implemented preview | F01, F02, F10 |
| [P06: Local runtime and inference (6/6B/6C)](phases/P06.md) | In progress; installer repaired locally, target integration pending | F03, F05 |
| [P07: Optional workspace](phases/P07.md) | Implemented preview | F06 |
| [P08: Application management](phases/P08.md) | Installation preview; wider lifecycle planned | F07 |
| [P09: In-place upgrades](phases/P09.md) | Source rollback hardened; target qualification pending | F08, F10 |
| [P09B: Trusted updates and release channels](phases/P09B.md) | Implemented preview; release blocked | F08, F04, F10 |
| [P09E: Signed artifact-bound approval](phases/P09E.md) | Merged PR #121; target qualification remains open | F08, F02, F04, F10 |
| [P09F: Build identity and update eligibility](phases/P09F.md) | Merged PR #123 (`d144cd7`); target qualification remains open | F08, F04, F10 |
| [P09G: Durable security auditing and attribution](phases/P09G.md) | Merged PR #125 (`fb18e46`); target qualification remains open | F02, F08, F10 |
| [P10A: Durable compute queue and admission foundation](phases/P10A.md) | Source complete; CI/merge approval pending, no live dispatcher | F09, F02, F10 |
| [NEXT: Scheduling, storage and release qualification](phases/NEXT.md) | Storage/admission foundation in P10A; runtime/API/UI and qualification remain | F09, F10 |

Recommended sequence: validate the repaired installer on WSL and fix updater release blockers first. Contract/dashboard reconciliation, read-only telemetry validation and packaging test preparation can proceed independently with file reservations. Trusted updates depend on proven base upgrade recovery; scheduling depends on an agreed durable storage contract. Full qualification follows integration of all release-scoped features. These are review recommendations, not assigned issues or deadlines.
