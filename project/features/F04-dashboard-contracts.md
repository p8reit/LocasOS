# F04 — Dashboard and API contracts

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Shipped embedded UI; React scaffold separate. **Suggested owner:** A5 Frontend + A2 Backend.

**Phase mapping:** Sprint 0 scaffold through current UI. **Dependencies:** F02, F03.

## Purpose and implemented scope

Embedded HTML dashboard implements enrollment, telemetry, models/chat, apps and updates. React hardware shell exists but calls /api/v1/hardware and is not bundled into the appliance.

## Evidence and owned files

`cmd/locas-core/{index.html,main.go}; web/src/; api/openapi.yaml; schemas/config.schema.json` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Decide whether to retain embedded UI or migrate React; document actual /api/* routes and error shapes; remove mock fallbacks from production paths; add shipped-dashboard integration coverage.

## Guardrails

Do not treat a React build as appliance UI validation. Shared API contracts and embedded dashboard require coordinated edits. Clearly label placeholder or unsupported actions.

## Acceptance gate

Runtime OpenAPI matches route/method/auth/body behavior; shipped dashboard passes enrollment, failure and reconnect checks.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.
