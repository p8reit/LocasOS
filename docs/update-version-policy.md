# Build identity and update eligibility

P09F / [LOS-122](https://github.com/p8reit/LocasOS/issues/122). This is the version-policy layer on top of [one-time signed artifact approval](update-approval.md), not a release or native OS qualification.

## Identity and provenance

Official WSL release builds receive `LOCAS_BUILD_VERSION` from the protected signed-release workflow's tag. The helper accepts only `vMAJOR.MINOR.PATCH`, requires that exact lightweight/annotated tag to resolve to HEAD, and requires clean tracked and untracked source. Components must fit the amd64 numeric policy and the complete ASCII tag must fit 63 bytes, leaving one byte for the persisted newline. Signed manifest validation uses the same numeric/length policy.

Both Core and updater embed the same tag and full source commit. Without an explicit release tag the helper emits `dev-<short commit>` and the full source-base commit, even when HEAD happens to have a tag. No-Git developer builds report `dev`/`unknown`. A development commit is a source-base identifier, not proof that a dirty tree is byte-identical to that commit. The native developer payload explicitly requests development identity even if a WSL release variable is inherited; it does not become a native production release.

`locas-core --version` and `locas-updater --version` print read-only JSON `{version, commit, release}` without starting services, reading administrator credentials or processing queued work. `release` is empty for development/opaque identifiers. These fields report embedded build metadata; publisher authentication still requires the existing signature/trust chain.

The dashboard/API keeps these meanings separate:

| Field | Meaning |
|---|---|
| `installed_version` | Raw version identifier of the running Core binary, retained for compatibility |
| `installed_commit` | Its embedded source-base commit, or `unknown` |
| `installed_release` | Its valid numeric release tag, or empty; never inferred from a record alone |
| `last_verified_release` | Root-owned `current-version` record from the last successful signed worker update, or empty |
| `version_basis` | `running_release`, `verified_record`, `unknown`, `inconsistent`, or `invalid_state` |

A record does not prove the running binary matches it. A valid running release and a different valid record cause `inconsistent`, blocking installation. This can be transient while an update has restarted the new services but has not yet completed health verification and atomic state persistence.

## Numeric policy

Core returns `comparison`, `availability`, `install_eligible`, and `eligibility_reason` with release discovery:

| Situation | Comparison / availability | Dashboard admission |
|---|---|---|
| Target numerically newer than known running release / verified floor | `newer` / `update_available` | Eligible, still requires signed metadata and one-time approval |
| Same known version | `same` / `up_to_date` | Blocked; promotion to another channel does not require reinstalling identical bytes |
| Older than known version | `older` / `newer_installed` | Blocked; switching to Stable does not silently downgrade |
| Opaque development/legacy build without a record | `unknown` / `comparison_unknown` | Existing explicit signed replacement path retained, with a warning that the target is **not confirmed newer** |
| Opaque build with a valid record | Compare against that record; basis `verified_record` | Do not claim the record identifies the running binary |
| Corrupt, unreadable, symlink, FIFO, directory or oversized record; malformed release-shaped build identity | `blocked` / `version_state_invalid` | Fail closed |
| Running release and verified record disagree | `blocked` / `version_state_inconsistent` | Fail closed pending completed update or verified recovery |

Versions compare numeric components, so `v1.10.0` is newer than `v1.9.0`. Early Access remains the release's prerelease visibility flag, not a version suffix. Unknown does not mean older. Neither a version label nor eligibility establishes archive authenticity: the signature, approved SHA-256, channel and OS-base checks remain mandatory.

Core re-evaluates state after discovery network I/O and before consuming an enqueue approval. A newly ineligible selection is invalidated and cannot be revived by a subsequent state repair. The independent worker applies the policy before downloading and again before invoking installation under its worker serialization lock, including fresh official images with no prior completed-update record.

Known same-version dashboard reinstalls are intentionally blocked, replacing P09E's previous same-version admission. The verified manual repair route remains an explicit maintainer operation. Do not run manual repair concurrently with a live/queued update; stop and inspect the update workflow first. Privileged manual intervention or arbitrary root edits are not covered by dashboard admission guarantees.

## Persisted state and recovery limits

The worker writes the new verified version only after the install command's service-health acceptance succeeds. It writes and fsyncs a temporary file, atomically renames it, then fsyncs the root-owned directory before reporting success. Readers therefore do not observe a truncation window. A persistence error reports failure, not installed success; binaries may already have changed, so operators must inspect actual signed artifacts, embedded metadata, service health and backups before verified recovery. Never resolve a mismatch by guessing or blindly editing the version record.

A missing record is a genuine legacy/fresh-image case. An unreadable or invalid record is not treated as absence. Linux reads are bounded, nonblocking, and reject final symlinks and special files. Root state remains separate from the unprivileged Core's channel preferences and temporary approval store.

This milestone does not supply a complete audit ledger, power-loss recovery certification, production signing/promotion, real Windows/WSL upgrade evidence or physical native installation. Those release gates remain open.
