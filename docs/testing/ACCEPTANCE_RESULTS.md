# Deployment acceptance record

Copy this template into a private test record for each build/target. Do not mark an operation passed because CI or documentation passed. Use a timestamp with timezone, such as an ISO8601 local timestamp with its UTC offset. Keep machine identifiers and full logs private; attach only reviewed/redacted excerpts to issues.

Statuses: **PASS** means executed and expected/observed results agree; **FAIL** means executed and they differ; **BLOCKED** means an identified prerequisite prevented execution; **NOT RUN** means not attempted. Record blocked and not-run cases separately from passes.

## Build and target identity

```text
Test record ID:
Operator / test timestamp and timezone:
Target: WSL development / signed WSL release / native diskless developer VM
Host OS/build, CPU/RAM/free storage:
WSL version, distribution name and test port (if applicable):
GPU/driver and VM passthrough availability (if applicable):
Reviewed source commit (full SHA):
PR head/base/synthetic merge (if using a PR artifact):
Workflow name / run URL / run ID / attempt:
Artifact name / ID / downloaded timestamp:
Archive or ISO filename / SHA-256:
Build-info commit/ref (WSL) / signed release tag and manifest identity (if applicable):
Guide revision used:
Private evidence directory / protected backup location:
Pre-existing appliances/workloads preserved:
```

Do not include passwords, cookies, bearer tokens, private keys, credential files, prompt/reply bodies or unredacted browser network captures. Public build IDs/digests are not secrets; audit references and machine inventory can still reveal private activity. Review diagnostic output before sharing.

## Results

| ID | Scenario / expected result | Status | Actual command/action and observed result | Private evidence / blocker / next action |
|---|---|---|---|---|
| D01 | Authentic reviewed workflow/artifact and exact intended source identified; checksum matches | NOT RUN | | |
| W01 | Fresh disposable WSL target; no existing distro/path/task or port collision | NOT RUN | | |
| W02 | Complete package imports; firstboot/Agent/Core healthy; Windows JSON identifies Core | NOT RUN | | |
| W03 | Dedicated browser profile; unique-password enrollment, login and logout work | NOT RUN | | |
| W04 | Cookie-free protected API probes return401; public session response is classified correctly | NOT RUN | | |
| W05 | Approved app/model actions complete or report accurate failure/uncertainty; no arbitrary execution | NOT RUN | | |
| W06 | Optional actual inference and processor/offload observed; telemetry alone is not GPU proof | NOT RUN | | |
| W07 | Core/Agent owner-user audit verification succeeds; exercised workers checked separately | NOT RUN | | |
| W08 | Console close, targeted distro restart, Windows sign-in and host reboot individually checked | NOT RUN | | |
| W09 | Optional newer signed update: trusted key, approval, independent verification, health and preservation | NOT RUN | | |
| W10 | Quiescent backup/export and external-data inventory recorded; restoration separately qualified | NOT RUN | | |
| N01 | Linux host inventory collected read-only; reviewed developer ISO checksum matches | NOT RUN | | |
| N02 | Diskless QEMU/OVMF boots to interactive guest; no host device/share/port passthrough | NOT RUN | | |
| N03 | Guest release identity, service readiness, loopback Core health and anonymous401 observed | NOT RUN | | |
| N04 | Native app/update workers remain disabled; guest shutdown normal; evidence retained | NOT RUN | | |
| Q01 | Separate Linux developer checkout at intended SHA: full unit/vet/module checks | NOT RUN | | |
| Q02 | P10A focused no-CGO and race tests pass using disposable test data | NOT RUN | | |
| G01 | Physical native installation/USB, persistent native state and recovery qualification | BLOCKED | No supported physical installer/procedure in this build | Track native gates; not a pass |
| G02 | Live scheduled GPU jobs, payload lifecycle, API/UI and physical process quiescence | BLOCKED | P10A is metadata-only and not wired into the appliance | Track F09 runtime integration; not a pass |
| G03 | Physical power-loss, interrupted install, full-journal/storage failure and disaster recovery | NOT RUN | Requires a separately planned disposable-target qualification | |

Replace each row's default status only after collecting evidence. Expand W08 into separate rows for each lifecycle event. Do not trigger an update, installer or corruption merely to populate a row. Worker journals may not exist until that worker has actually run; record the unexercised path rather than reporting an empty valid history. A dev build without a configured trusted update key is an honest W09 blocker.

## Summary and reproducible handoff

```text
Executed PASS / FAIL / BLOCKED / NOT RUN counts:
Observed user-visible behavior:
Failures with shortest exact reproduction:
Pending or uncertain operations and reconciliation evidence:
Data preservation / backup verification / restore status:
Actions deliberately not taken:
Remaining native/runtime/release gates:
Next exact action, responsible role and prerequisite:
```

A WSL export is not a complete backup of external mounts, Windows files, Docker/remote storage or every application dependency. A VM firmware/health smoke is not a physical hardware certification. Do not retire/reset an environment until the exact disposable target, preserved evidence, backup and external-data scope are explicitly confirmed. See the [deployment hub](../TEST_DEPLOYMENTS.md), [WSL guide](WSL_DEPLOYMENT.md), [physical-host guide](PHYSICAL_HOST.md), [queue contract](../architecture/compute-queue.md) and [audit recovery](../security/auditing.md).
