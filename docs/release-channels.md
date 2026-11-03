> Review notice (2026-11-02): This document includes historical plans or implementation claims that predate checkout `9f2aabe`. Consult the [current feature and phase map](../project/README.md) for reviewed status, remaining work and guardrails. Existing release/security requirements still apply; this notice does not certify them as satisfied.

# LOCAS OS releases: test what you ship

LOCAS OS uses **one artifact path** for early testing and public releases.
There is no separate dev appliance, branch-specific image, or reimport requirement.

## Lifecycle

1. Tag the approved commit as `v0.10.0` and push the tag.
2. `signed-release.yml` uses the normal WSL build and creates an **early-access GitHub prerelease** of that version, including the upgrade archive and its signed manifest.
3. Install that release on TowerDoom using `Upgrade-LocasOS.ps1 -Apply`. It upgrades the running dedicated WSL distribution without deleting models or Open WebUI data.
4. After validating application health, models, and upgrade rollback, manually run **Promote LOCAS Beta to Stable** with the exact existing tag. Protect the `locasos-stable-promotion` GitHub Environment with required reviewers.
5. Promotion only clears the GitHub prerelease flag. **No rebuild, re-sign, new tag, or asset replacement** occurs. This guarantees stable customers receive the same tested bytes.

Version tags are `vMAJOR.MINOR.PATCH`. The *release state* (prerelease or stable), not a `-beta` version suffix, determines the update channel so a prerelease can be promoted unchanged.

## Release key and protections

The release workflow refuses to publish if the protected GitHub Actions secret `LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64` is missing. Provision an Ed25519 private key (64-byte standard Go representation, base64-encoded) in the `locasos-releases` GitHub Environment. Restrict that environment to trusted maintainers and protected branches/tags. Back up the secret securely; never commit it.

**Before enabling in-dashboard privileged installation**, pin its corresponding Ed25519 public key in the appliance, then verify the signature against the raw `locasos-update.json` bytes, the signed archive SHA-256, the release version, expected platform and allowed file paths. An attacker-controlled key from the same release is never trusted. This first delivery implements signing and verification primitives and a promotion workflow, **not** automatic privileged installation.

GitHub Releases is the only public distribution endpoint. End-user chats, Ollama models and private application data are never uploaded.

## Testing

For an early build, use the existing appliance; do not reinstall or run `Install-TestPackage.ps1`.
```powershell
.\Upgrade-LocasOS.ps1 -Apply
.\Test-LocasOS.ps1
wsl -d LocasOS -u root -- /usr/local/bin/ollama list
```

The early-access channel is opt-in. Stable should remain the default when LOCAS adds the channel selector.

## Promotion verification configuration (2026-11-03)

The protected `locasos-stable-promotion` environment must define the public variable `LOCAS_RELEASE_ED25519_PUBLIC_KEY_B64`, containing the base64 Ed25519 public key corresponding to the separately held signing secret. This key must be provisioned independently by a trusted maintainer; never copy a key from downloaded release assets. Missing/invalid configuration blocks promotion. No environment configuration was changed by this implementation.

Promotion now downloads the existing signed manifest, signature and archive, verifies raw manifest bytes and archive digest with `locas-release verify VERSION COMMIT DIST`, and requires the signed commit to match the tag. It checks that release asset metadata did not change during verification, then clears only the prerelease flag. This does not replace the required device test, reviewer approval or repository policy forbidding asset/tag mutation. GitHub's promotion call is not an atomic compare-and-swap against asset contents; repository protections and trusted maintainers remain necessary.
