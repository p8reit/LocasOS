# LOS-130 — Ollama / Open WebUI install failure

Status IN PROGRESS. Issue [#130](https://github.com/p8reit/LocasOS/issues/130), branch `agent/platform/LOS-130`, base `c94021d`. Operator tested build `a91e9e2` (WSL run41827900692) and now reports install failure with `uncertain:open-webui`. Preserve earlier installation success, but app/runtime acceptance is now FAIL / investigation, not an all-features qualification.

Goal: identify reproducible installer/worker/readiness defects, improve classified diagnostics and actionable status, preserve honest partial-effect uncertainty, secrets, journals and application data. Operator journals confirm repeated Ollama bind failures on 127.0.0.1:11434; read-only Windows checks identify Windows Ollama on that port. No raw logs, private host names or executable paths are published. Source changes must not be presented as a confirmed machine fix without evidence.

Owners: lead worker/status/UI integration; platform agent installer scripts and isolated shell regression tests; independent A6 review. No direct appliance access, destructive reinstall, journal deletion, Docker Desktop use, production deployment, signature bypass or release/signing authorized. Root worker executes only fixed commands and typed attributed jobs.

Next: inspect raw worker/Ollama logs supplied by operator, reproduce source issues in disposable tests, implement guarded fixes and checks; preserve evidence. Record exact validation/commit/CI and review before a new merge approval gate.

## Implementation checkpoint

Completed source work: explicit Debian docker-cli dependency; bounded HTTP health and service/socket checks; safe fixed diagnostic stages and correlation references; snapshot-bound acknowledgement before uncertain retries; preserved models, WebUI container/data/key and audit journals; optional strict loopback OLLAMA_HOST shared by Core/Agent/worker/Ollama through root-owned runtime.env. Default unchanged; no foreign listener stop or automatic migration. Existing WebUI endpoint mismatch blocks rather than rewrites configuration. Unprivileged readiness establishes service-account ownership, not exact PID socket attribution; root worker/installers enforce exact socket ownership. No privilege relaxation.

Guide: docs/testing/APP_INSTALL_TROUBLESHOOTING.md. Native remains developer-only. runtime.env excluded from payloads and preserved through upgrade/rollback. Manual wrappers do not inherit systemd configuration.

Validation on Linux: full go test ./... PASS; go vet ./... PASS; focused appmanager/worker race PASS; Node 51 PASS; isolated actual-script installer tests 34 PASS; upgrade/recovery 16 PASS; build identity 9 PASS; shell syntax PASS. No real Windows/WSL installation, GPU inference or appliance recovery executed by agents. User's live runtime/app acceptance remains FAIL pending retest.

Agents: installer implementation delivered and independently reviewed by existing worker/reviewer (requested gpt-6.1-sol high; actual routing/cost metrics unavailable). Final packaging/docs delegation failed because subagent usage limit was reached; lead completed those paths. Independent reviewer examined runtime readiness source with no blocker, but full final integration review remains pending. No merge/deploy authorized for this new change.

Resume: branch agent/platform/LOS-130, base c94021d. Commit/push source, create issue-linked draft PR, inspect CI and obtain final review, then request explicit merge approval under AGENTS.md section 3. Operator retest uses the complete updated bundle, never partial older binaries.
