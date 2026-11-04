# LOS-128 — WSL and physical-host test deployment guides

Status: documentation implemented and independently reviewed; local checks/PR CI evidence and separate merge approval are required before integration. Issue [#128](https://github.com/p8reit/LocasOS/issues/128), branch `agent/docs/LOS-128`, base `a91e9e2` (approved P10A PR #127 merged).

## Goal and acceptance

Provide practical source-checked deployment instructions for testing current merged features in WSL and on physical development hosts, with artifact provenance, prerequisites, exact host/guest contexts, expected results, privacy, diagnostics and nondestructive stopping. Preserve the supported boundary: WSL can run on physical Windows hardware; native physical-disk/USB installation is unsupported, so native evaluation is read-only host readiness plus a diskless VM. Distinguish P10A metadata tests from live GPU scheduling and CI checks from operator/hardware qualification.

Deliverables: `docs/TEST_DEPLOYMENTS.md`, `docs/testing/{WSL_DEPLOYMENT,PHYSICAL_HOST,ACCEPTANCE_RESULTS}.md`, links from README/existing deployment guides and feature memory. No application code, installer behavior or release path is changed. No actual deployments, downloads/models, WSL registration, USB/disk writes, production data changes or releases are authorized/performed by this documentation task.

## Ownership and review

- WSL guide: `wsl_deployment_guide`, parallel writer; requested gpt-6.1-sol/medium.
- Physical-host guide: `physical_deployment_guide`, parallel writer; requested gpt-6.1-sol/high for disk/boot boundaries.
- Hub/results template, source reconciliation, CI/docs checks and integration: lead.
- Independent read-only A6: `deployment_docs_review`, requested gpt-6.1-sol/high. No blocking findings; suggested explicit native disabled/inactive worker checks were added.

Workers used disjoint files. Actual provider-internal model routing and token/cost metrics are unavailable; requested selectors are recorded without savings claims. No workers deployed or changed host state.

## Decisions and guardrails

Clean Windows VM/spare machine is the default development WSL route because current standard helpers only accept `LocasOS`/59420. Signed side-by-side release helper limitations are documented, including transient first-launch default port and host-scoped cookies. Require a separate browser profile. Distinguish signed release/rootfs, unsigned Actions artifact, upgrade tar and developer ISO. PR head and synthetic merge SHAs differ; main build provenance is preferred for merged-source acceptance. Document expiry and fresh-build selection rather than stale run IDs.

Native guide permits only read-only host inventory and an isolated graphical, diskless/networkless QEMU guest with fresh firmware variables, no passthrough/shares/forwarding. It provides guest-local HTTP/UID checks, preserves loopback, does not assume default credentials/sudo/SSH, and verifies disabled worker paths. Physical installer/authentication/recovery/GPU gates remain open.

P09G journal verification runs as writer UID and preserves evidence; absent unused-worker journals are not fake passes. P10A requires Linux developer source tests; appliance binaries have no queue executor/API/UI or Go SDK assumption. Results template requires PASS/FAIL/BLOCKED/NOT RUN and actual source/artifact/machine evidence without secrets. No automatic unregister/reset or destructive cleanup command is provided.

## Validation and resume

Independent review: relative file links and balanced fences PASS; eight Bash examples parse PASS without execution; workflow YAML/literal Bash checks PASS; final independent delta review CLEAR. PowerShell parser absent locally so local parsing NOT RUN. CI adds parsing only, never execution, for the guide's PowerShell examples. Exact final local/remote outcomes are recorded on the issue-linked PR before readiness.

NOT RUN: Windows/WSL commands, model/runtime downloads, QEMU boot or physical hardware, native installation/USB, GPU inference, actual backup/restore, production signing/release/deployment. Source/docs review does not certify any of those.

Resume: read this ledger and `AGENTS.md`, `git status --short`, issue #128 and its PR. Next: verify the exact PR head/checks, obtain explicit merge approval, then the operator selects a disposable target/build and records actual acceptance. The guide does not authorize replacing a physical OS or changing production state.
