# LOCAS OS update rollout plan: one build, early access first

## The rule

**One release candidate, one immutable signed package, two visibility states.**
A version is built and signed exactly once, published as an **Early Access** GitHub prerelease, tested on normal LOCAS OS machines, and later promoted by changing the **same GitHub Release** to a normal Stable release. Promotion never recompiles, rearchives, retags or re-signs the payload.

No parallel test appliances or separate beta Docker stack are required. TowerDoom can stay a regular LocasOS WSL installation with its existing models and data.

## User channels

| Channel | Opt-in | GitHub source | Install |
|---|---|---|---|
| Stable | Default | Non-prerelease, published version | Explicit admin approval |
| Early Access | Yes | Published GitHub prereleases plus stable releases | Same exact updater, explicit admin approval |
| Developer artifact | Not exposed to normal users | CI jobs | PowerShell/manual only |

A user switches between Stable and Early Access in Settings > Updates; this changes *discovery only*, not the installed instance or data. Channel changes do not silently downgrade an installation. Stable and Early Access must use the same signature verification, download staging, health checks and rollback.

## Release and promotion flow

1. Merge and validate a candidate commit.
2. Create an immutable release tag `vX.Y.Z`. Run the signed release workflow to build **once** and publish it as GitHub prerelease with notes and exact signed artifacts.
3. Test Early Access on the **same existing WSL LocasOS distro**, preserving locally stored Ollama models, Open WebUI data, Docker volumes, and login data.
4. Verify update status, health, reboot/autostart, backup/rollback, Ollama inference and dashboard connectivity on TowerDoom.
5. Approve the protected manual promotion workflow. It must verify manifest/signature/artifact digest and change only release `prerelease` metadata to false; never modify release assets.
6. Stable users discover those identical archive bytes by the same tag and SHA-256.

For issues, leave the release as prerelease, publish a **new version/tag and package** after fixes, and retest. Never silently replace or re-upload package files under an existing release.

## Acceptance gates before dashboard one-click install

- Pinned public release-signing key inside the appliance, never fetched dynamically from a release.
- Verify Ed25519 signature on original manifest bytes, digest of the archive, supported version and Linux/WSL target; enforce maximum download size, timeouts, and strict GitHub asset allowlist.
- Bind admin approval to a specific version + digest and an explicit one-time job; never pass arbitrary URLs or commands to root.
- A separate systemd update worker performs bounded downloading, backup, tar-entry validation, extraction, service restart, health verification, persistent progress and automatic rollback.
- Manual `Upgrade-LocasOS.ps1` remains the break-glass recovery route.
- CI tests stable/prerelease filtering, signature tampering, malformed/oversized archive, interrupted transfers, persisted status, install failure, rollback and data preservation.
- TowerDoom verifies the actual Early Access archive *before* the exact same archive is promoted to Stable.

## Current implementation vs remaining work

- Signed manifest tooling and GitHub prerelease publishing, plus protected promotion workflow: implemented in repository, but signing secret/key provisioning and the actual first GitHub release are not confirmed.
- Dashboard release lookup and manual assets: implemented.
- Dashboard channel selector and installed-version comparison: **not implemented**.
- Runtime public-key pinning, signed update verification, download/stage worker, approval-to-root boundary, progress UI, full rollback: **not implemented**.
- Promotion workflow currently checks asset presence, but must validate the signature and SHA-256 before it is safe for production use.

Do not claim auto-update or fully signed verification works until the remaining gates are built and tested.
