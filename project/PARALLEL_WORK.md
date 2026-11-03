# Parallel component work and recall

Read [the index](README.md), the relevant feature file, its phase file and [agent policy](AGENT_POLICY.md) at task start. This workflow adds coordination; it does not relax existing merge/security gates.

## Assignment record

Before implementation, record in the issue/PR: feature ID, objective, owner, branch `agent/<role>/LOS-NNN`, base commit, reserved paths, shared contracts, dependencies, acceptance criteria and blockers. Current review assigned no implementation workers. Specialist role definitions are in `agents/`; they are not running services.

Use separate branches/worktrees per component. In a shared checkout, reserve disjoint files before editing and integrate sequentially. One integration owner handles shared entrypoints. A draft PR must link the issue; human/integration approval is required before merge. Do not create fake issue numbers or claim approval.

## Practical boundaries

| Workstream | Suggested primary files | Coordinate with |
|---|---|---|
| Appliance | Windows lifecycle scripts, rootfs and first boot | Security/service identities; updater packaging |
| Security | admin, peercred and negative tests | All mutating handlers |
| Telemetry | hardware, gpu, docker collectors | Agent API and dashboard contracts |
| Runtime/apps | installers, appmanager and app worker | Service units and runtime proxy |
| Updates | updatetrust, updatequeue, updater/release CLI | Packaging, root worker, workflows and dashboard |
| UI/contracts | Embedded dashboard or explicitly chosen React migration; OpenAPI | Every API producer |
| Scheduling | New scheduler/storage packages after contract agreement | GPU policy, API and persistence |

`cmd/locas-core/main.go`, `cmd/locas-agent/main.go`, `cmd/locas-core/index.html`, `packaging/wsl/build-rootfs.sh`, service units, workflows and OpenAPI are shared integration files. Reserve them explicitly; do not independently rewrite them from multiple components. Consumer and producer agree route/schema/error/auth changes before implementation.

## Validation and handoff

Run checks appropriate to changed code: Go tests/format, Bash parsing for every changed shell script, React check/build only for React, shipped dashboard/API behavior for embedded UI, PowerShell parse and actual Windows/WSL tests for lifecycle changes. Never run provisioning/reset scripts on the review host merely to test syntax.

End each task by updating its feature file: source commit, issue/PR, changed paths, exact tests/results/environment, remaining blockers and next concrete action. Update phase/index status when acceptance changes. Re-run integration after shared-file merges. Use REVIEW.md finding IDs in issues to retain context.

Record actual agent/model identity and available usage/attempt metrics; mark unavailable metrics unavailable. No cost-savings claims without measurement. Narrow documentation/tests can use cheaper models; architecture/concurrency/security work warrants stronger review. This review used three specialist subagents with the inherited model; per-agent token/cost metrics were unavailable.
