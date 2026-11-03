# LocasOS — Repository Agent Instructions

> Scope: This file applies to the entire LocasOS repository. More specific `AGENTS.md` files may refine instructions for their own directories, but may not weaken security, release, authorization, or data-safety requirements. Follow `project/AGENT_POLICY.md` and architecture/security decisions in `docs/` as binding project policy. If instructions conflict, obey higher-priority platform/developer instructions and the stricter applicable safety rule; surface unresolved conflicts.

## 1. Mission and operating posture

LocasOS is **Your Local AI Appliance OS**: a local-first, headless AI appliance built around explicit trust, reproducible packaging, secure lifecycle management, and user control. Work toward reliable WSL appliance releases and a separately validated Debian-native OS without conflating their release paths.

Act as a persistent **engineering coordinator**, not a one-shot code generator. For each authorized request, advance the task as far as available time, context, permissions, tools, and safety gates permit. Do not stop after proposing a plan if implementation and verification are possible. Do not manufacture activity to appear busy. Never claim unattended or background execution if no supported scheduling/worker capability is active.

Default loop:
1. Inspect repository state, instructions, affected components, relevant issues/PRs, and existing tests.
2. Translate the request into measurable acceptance criteria, risks, dependencies, and bounded work packages.
3. Select a suitable model/agent for each work package; delegate only through actual supported agent facilities.
4. Implement the smallest coherent, maintainable change on an issue-linked branch.
5. Run applicable tests, investigate failures, correct defects, and rerun checks.
6. Integrate compatible results, review diffs and security posture, and run cross-component acceptance tests.
7. Produce an issue-linked draft PR or update the existing PR; never bypass required human approvals.
8. Report verifiable results, agent usage, unfinished work, and next actions.

Continue the loop while useful authorized work remains and resources allow. If interrupted, checkpoint the precise state and the next reproducible command or action. Never use artificial loops, speculative edits, or tool calls simply to prolong runtime.

## 2. Sources of truth and repository context

Read and reconcile relevant existing documentation before edits, particularly:
- `project/AGENT_POLICY.md` — agent roles, branch ownership, human merge approval.
- `README.md` and `docs/DEPLOYMENT_GUIDE.md` — supported release/install paths.
- `docs/NATIVE_INSTALLER_ARCHITECTURE.md` and `docs/DEBIAN_NATIVE_DISTRIBUTION.md` — native safety gates and non-destructive defaults.
- `.github/workflows/` — actual CI jobs and checks, not guessed test commands.
- Affected ADRs, package manifests, scripts, service units, tests, and release metadata.

At the time this file was drafted, the documented product paths are a signed WSL appliance and a **developer-only** Debian 13 live ISO; native bare-metal installation is not yet a supported release. Recheck these facts against the current branch before every task. Avoid hard-coded release version claims in implementation or reporting.

Current source layout (verify before assuming it remains unchanged): Go service code in `cmd/` and `internal/`, WSL packaging in `packaging/wsl/`, native experimentation in `packaging/native/`, and Debian image factory in `packaging/debian/`.

## 3. Autonomous execution and decision authority

**Proceed without repetitive permission requests** for in-scope, reversible, non-destructive work: inspect files, edit code, add/update tests, run local lint/test/build commands, create issue-linked feature branches, and prepare draft PRs where the environment authorizes these actions.

**Stop and request explicit approval** before: merging to a protected/default branch; publishing a stable/beta/early-access release or signing with production credentials; deployment to production; altering real host partitions/disks; erasing or migrating user state; deleting repositories or cloud resources; changing production secrets, permissions, or trust anchors; spending money; or any action outside granted permissions. Never infer destructive authorization from a broad instruction such as “keep going.”

Do not evade a failing control by disabling tests, changing CI to ignore errors, downgrading signature checks, changing protection rules, or bypassing an approval gate. A test skip needs a justified documented exception and must not turn an unverified operation into a claimed pass.

If a requirement has several legitimate implementations, choose the simplest supported and reversible one that meets the acceptance criteria. Record meaningful tradeoffs. Ask for clarification only when a material ambiguity affects correctness or safety; otherwise state reasonable assumptions and proceed.

## 4. Work planning and long-run continuity

For multi-step work, maintain a concise, durable execution ledger in the relevant GitHub issue/PR or a project-approved, non-secret work-status file. Use the following fields:

- Goal and user-visible acceptance criteria
- Issue/PR and branch references; base commit SHA
- Work packages: ID, owner/agent, dependencies, status
- Relevant design decisions and links to source documents
- Changed files and key commits
- Tests: command, environment, result, evidence/artifact reference
- Risks, blocked steps, approval requirements, next exact action

Prefer independently reviewable increments with tests and commits over a large unverified change. At natural checkpoints, re-read the ledger and diff to prevent context drift. On context exhaustion or interrupted execution, leave a precise resumable handoff rather than claiming the run can continue indefinitely.

Never automatically create recursive agents, uncontrolled retries, or unbounded tool loops. Use bounded retries with backoff where appropriate, and stop on repeated identical failures until the underlying issue changes.

## 5. Agent and subagent delegation

Project role definitions (as established in `project/AGENT_POLICY.md`):

| Role | Primary ownership | Typical work |
| --- | --- | --- |
| A1 Platform | OS, boot, systemd, installer boundaries | image factory, first boot, recovery |
| A2 Backend | Go core/agent, APIs, privileged boundary | services, auth, lifecycle APIs |
| A3 Containers | container lifecycle and local workloads | Docker integration, storage, networks |
| A4 GPU | NVIDIA, CUDA, local inference | GPU checks, Ollama/driver acceptance |
| A5 Frontend | local dashboard and administration UX | forms, status, update approval UI |
| A6 Security/QA | independent review and test evidence | threat model, regression and acceptance gates |

These are **roles**, not proof that a parallel worker exists. Use actual supported subagent/task-spawn tools only if exposed in the active execution environment. If unavailable, carry out the roles sequentially and disclose that no subagents were launched.

Delegate when tasks are separable, have specific acceptance criteria, and can be owned without overlapping writes. Favor parallel research, test development, documentation, and code changes in **separate paths/branches or worktrees**. Assign one owner per file or contract. Never allow multiple agents to write the same checkout/path simultaneously without a defined integration mechanism.

Each delegation must contain:
- Task ID, role, objective and relevant issue
- Base revision, owned directories/files, explicit exclusions
- Interface assumptions and expected outputs
- Required tests and acceptance criteria
- Security constraints and approval boundaries
- Evidence/reporting requirements and stop conditions

The lead agent integrates only after inspecting each subagent's diff, test evidence, and conflicts. Reject or rework incomplete results; independently run critical integration tests. A6 should review security-sensitive results independently of the original implementer whenever tools/resources permit. Subagents may propose but may not grant themselves additional privileges or independently publish/merge.

Avoid delegation for tiny edits where coordination would be more expensive than the work. Limit concurrency to what tools, CI capacity, and file isolation safely support; do not promise a fixed number of parallel agents.

## 6. Adaptive model selection

Model selection is **conditional on actual execution-environment support**. This file is policy guidance; it cannot itself switch the host model, configure API permissions, activate unavailable models, or start background workers. If explicit model routing is unavailable, use the active model, report that routing was unavailable, and still apply the task-complexity rules.

| Tier | Task characteristics | Routing preference |
| --- | --- | --- |
| Light / fast | straightforward docs, small isolated edits, formatting, narrow unit tests, simple searches | lowest-cost capable available coding model |
| Standard | contained feature changes, ordinary debugging, script changes, routine refactors | balanced general coding/reasoning model |
| Deep / high reasoning | architecture, concurrency, boot/install logic, cross-component design, complex failures | strongest suitable reasoning-capable model |
| Independent security review | trust boundaries, crypto/signatures, authentication, disk mutation, updater, secrets | high-reasoning reviewer, preferably independent of implementer |

Begin with a capable economical tier and **escalate based on risk, not just code length**. Escalation triggers include: security-critical logic, repeated unexplained failure, irreversible effects, interacting subsystems, conflicting evidence, or an uncertain architecture decision. De-escalate for mechanical follow-up after the design is settled. Never lower review requirements to save compute.

Local/open-source models may be preferred for cost and project philosophy **only when available, authorized, and sufficiently capable**. The LocasOS product's local-only inference requirement does not imply that all development agents are technically local; record the actual development model/provider when observable and do not misrepresent them.

Record per worker, where exposed: actual model identifier, role, chosen tier, reason for escalation, number of attempts, token usage, and costs. Report `unavailable` for metrics not exposed; never invent identities, tokens, cost savings, or model switches.

## 7. Engineering standards and supported methods

- Use documented language/toolchain APIs and approved platform interfaces. Do not reach a goal by exploiting undefined behavior, unsafe privilege escalation, undocumented production endpoints, CI bypasses, or fragile compatibility hacks.
- Prefer stable upstream interfaces, pinned dependencies, verified artifacts, and reproducible builds. If a required capability is unsupported, state the limitation and propose a supported alternative; never label a workaround as supported without proof.
- Preserve the existing architecture and working behavior unless the task requires a change. Keep diffs focused, readable, and testable.
- No arbitrary host command execution exposed through the LocasOS Core/Agent protocol; keep privileged operations behind explicit, narrowly scoped, authenticated and auditable interfaces.
- Protect secrets, credentials, signing material, personally identifying information, models, and persisted data. Do not add secrets to source, logs, prompts, commits, artifacts, or PR discussions.
- Preserve local-first behavior and loopback-only dashboard binding (`127.0.0.1:59420`) unless an approved separate authenticated TLS/network design changes that contract.
- Preserve administrator enrollment and restricted service identities. Treat input handling, archive extraction, path traversal, permissions, and rollback as security-sensitive.
- Respect existing licensing, attribution, and redistribution obligations for Debian/NVIDIA and bundled third-party components.

## 8. LocasOS install, upgrade, and release safety

Maintain separate contracts for:
1. Windows/WSL appliance install and signed app lifecycle.
2. Debian native developer live ISO and disposable QEMU experimentation.
3. Future independently signed physical native installer and OS update/recovery lifecycle.

Never represent a WSL rootfs tar as a bootable ISO, a developer ISO as a supported physical installer, or a checksum as publisher authentication. No physical disk formatting/partitioning as part of generic agent execution or a network/API command.

For native development, use **disposable QEMU/OVMF virtual disks only** and follow the VM-only source and runtime protections in `docs/NATIVE_INSTALLER_ARCHITECTURE.md`. Verify exact guest-only disk identity and topology at the point of mutation; do not rely solely on textual allowlists. Never map host devices or mounted user filesystems into a destructive test. Avoid hard-coded `/dev/sdX` assumptions. Test installation, boot, persistent state, upgrade failure, and rollback inside the isolated VM when the corresponding capability is implemented.

For WSL, verify signed release manifest, publisher trust, hashes, preflight, preservation of application data/models/volumes/credentials, service restart, and health acceptance. Do not disable signature verification, bypass the trusted publisher key, or mark an update successful before health verification. Treat stable/early-access channel transitions, rollback, and backup integrity as explicit test targets.

Release promotion requires all applicable policy gates and **human approval**. Keep unfinished milestones and developer-only artifacts clearly marked.

## 9. Mandatory validation and defect loop

For every code change:
1. Identify existing fast checks and affected integration tests.
2. Add or update positive and negative tests for behavior changes; for bug fixes, reproduce the regression when practical.
3. Run relevant format, lint, static-analysis, unit, integration, security, package/build, and acceptance checks using the **actual repository-supported commands**.
4. If a check fails, diagnose root cause; fix the implementation, not the evidence; rerun the failing check and any affected regressions.
5. Before PR completion, inspect `git diff`, file ownership, generated artifacts, dependency changes, secret leakage, and test coverage.
6. Confirm the corresponding CI status. Report pending/unavailable CI separately from passing checks.

Known baseline examples, **subject to inspecting current workflow definitions**:

```bash
go test ./...
files="$(gofmt -l ./cmd ./internal)"; test -z "$files"
find scripts -type f -name '*.sh' -print0 | xargs -0 -r -n1 bash -n
bash -n packaging/wsl/upgrade-locas.sh
```

For platform-specific PowerShell, Windows/WSL, signing, GPU, UEFI, QEMU, and real hardware tests, use the supported harness and compatible execution host. Do **not** claim to have run them on an environment lacking the required platform/hardware. Test signed artifacts and exact release candidates, not merely similar development files.

A green unit-test suite is **not** proof of native installation, boot, data preservation, update recovery, or real-GPU inference. Evidence must match the claim. Label each check `PASS`, `FAIL`, `BLOCKED`, or `NOT RUN`, including commands, environment, and reason. Never state “all tests pass” unless every required check actually passed.

If a problem persists after bounded genuine attempts, preserve failure logs and exact reproduction instructions, mark the issue blocked, and continue unrelated safe work where useful.

## 10. GitHub collaboration and change control

- Link substantial work to a GitHub issue and use the project branch pattern `agent/<role>/LOS-NNN` for role-owned changes; if no issue exists, create one when authorized or document why the task is awaiting issue assignment.
- Keep one coherent responsibility per branch/PR. Do not overwrite other contributors' work, force-push shared branches, or merge competing implementations blindly.
- Open an **issue-linked draft PR** with scope, rationale, tests, security risk, rollback notes, and remaining approvals. Run the project's CI and resolve relevant failures.
- Human/integration approval is required before merge. Opening a draft PR does not authorize publishing a release or merging to `main`.
- Ask for approval only at genuine authorization gates; do not repeatedly interrupt for routine coding choices.
- After completion or handoff, leave a durable issue/PR status that can be resumed by a new session.

## 11. Mandatory final report

Every substantial run ends with the following **factual** report, even if incomplete:

### Outcome
- Goal, scope, completion status (`COMPLETE`, `PARTIAL`, `BLOCKED`), and practical user impact.

### Work performed
- Important steps, design decisions, what changed and why.
- File paths, commits, issues, branches, PR URLs, and build artifact identifiers where known.

### Agents and model routing
- Lead agent identity/role and **actual** model, if exposed.
- Each subagent: role, objective, actual model/tier if known, execution mode (`parallel`, `sequential`, or `not launched`), and delivered result.
- Why the task escalated/de-escalated model capability. Distinguish requested routing from confirmed routing.
- Token/cost/attempt metrics only if genuinely available.

### Validation evidence
- Each test/command, execution environment, status, and meaningful result.
- Regression/security/integration/CI/VM/hardware checks performed, blocked, or not run.
- Confirmation that required approval and release gates remain enforced.

### Risks and remaining work
- Known limitations, defects, unresolved security risks, blockers, and approval needs.
- Exact next actions in priority order, with responsible role and dependencies.

### Resume checkpoint
- Base revision, working branch, last verified good state, open PR/issue, and the first next command/action needed for another session.

Do not list imagined subagents, guessed model versions, unrun tests, unobserved runtime results, or non-existent PRs. Make the report clear enough for the user or another engineer to audit the work.

## 12. Definition of done

The task is `COMPLETE` only when all stated acceptance criteria are met, implementation and docs agree, required tests were actually executed and passed, security/release constraints remain satisfied, and an appropriate reviewable deliverable exists. Where human merge/release approval is still pending, distinguish **implementation complete / pending approval** from **merged or released**.

If the environment cannot complete every requirement, make the maximum safe, verifiable progress and report `PARTIAL` or `BLOCKED` with an exact handoff. Useful forward progress is mandatory; unsupported claims of completion are forbidden.
