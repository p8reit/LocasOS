# LOCAS OS Release Channels and Test-to-Stable Promotion

## Decision: one build, two visibility channels
LOCAS uses GitHub Releases as its software distribution backend. **Early Access** is GitHub's prerelease state; **Stable** is the same GitHub Release after validation and promotion. There are no separately compiled beta/prod executables and no separate test distribution, model store, or virtual-machine lab.

A maintainer pushes one signed version tag, e.g. `v0.10.0`. The release workflow creates exactly one signed archive and matching manifest, publishes it as **prerelease**, and blocks publication if required signing secrets are unavailable. The user selects **Early Access** in the *same dashboard* installed on their existing WSL LocasOS appliance, installs the update through the same update engine, tests it, and reports success. Promotion only flips that release from prerelease to stable. **Do not rebuild or resign while promoting.** Identical tag, source commit, archive bytes, digest, and signature move to Stable.

### Channels
- **Stable** (default): newest published non-draft, non-prerelease supported signed release.
- **Early Access** (opt in): newest published signed release, including prereleases.
- **Development**: unpromoted GitHub Actions artifacts; *not* an in-appliance channel and never silently installed.

The current dashboard's release discovery does not yet implement channel selection, signature verification, or direct installation. Those remain next gated changes.

### Publishing and promotion
1. Commit passes Go tests, syntax checks, archive-manifest safety check, signing verifier tests, and WSL artifact build.
2. Release maintainer creates the annotated final version tag `vX.Y.Z` (beta status lives in release metadata rather than in the signed version; no retagging).
3. Protected GitHub Environment `locasos-releases` exposes `LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64` only to authorized release jobs. Publisher public key is pinned in LOCAS builds; never generated during an update, never downloaded from the same release.
4. Release workflow builds exactly once and attaches `locasos-upgrade-amd64.tar`, `locasos-update.json`, `locasos-update.sig`, and manual recovery scripts as prerelease assets.
5. Early Access devices test the *actual release artifact* via the normal appliance upgrade path; production user data is retained. Keep `Upgrade-LocasOS.ps1` as an offline recovery option.
6. Explicitly approve promotion using the manual promotion workflow and its protected environment. It verifies all signed assets and then executes only `gh release edit TAG --prerelease=false`—no rebuild, repackaging or re-signing.
7. A revoked release must be taken out of channel discovery, and devices must reject untrusted/incorrect manifest signatures. A stable release cannot be automatically downgraded to an earlier version.

### Runtime update security acceptance gates (before enabling one-click)
- Authenticate admin and require deliberate approval of release **tag and archive digest**.
- Resolve only `p8reit/LocasOS` release API and allowlisted asset names, verify signed manifest with embedded key, verify platform, version, digest and exact source commit, enforce maximum size and download timeout, never trust checksum alone.
- Download into a root-owned bounded staging directory, protect against symlinks/path traversal, validate archive entries and service modes before extraction.
- Root update worker runs independently of `locas-core` and survives core restart; only typed, allowlisted update requests can reach it. Persistent, permission-restricted progress and failure logs are queryable by the dashboard.
- Snapshot changed binaries and all replaced service units, apply upgrade, health-check Agent and Core, rollback on failure, verify rollback health; keep models, Docker and account state untouched.
- CI integration exercises malformed/unsigned packages, wrong channel/platform/digest, malicious tar entries, stalled downloads, upgrade/rollback against a disposable Linux service environment, and a smoke test for preserving data.
- Test both **Early Access** and **Stable** selection against the *same tag*, and compare download asset SHA-256 before and after promotion.
- Manual install and WSL export remain supported, with explicit Windows validation on TowerDoom before rollout.

The package-signing implementation is step one. Do not claim the full one-click lifecycle exists until all gates pass.
