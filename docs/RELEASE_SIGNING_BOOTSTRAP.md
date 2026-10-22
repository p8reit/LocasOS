# One-time release publisher key setup

This operation is for the repository maintainer, **not** for every LocasOS user.
Perform key generation locally on a trusted machine. Do not share the private key
with ChatGPT, put it in a commit, upload it as an artifact, or print it in CI logs.

## Generate and back up the key

With Go 1.23+ and a trusted checkout:

```bash
umask 077
go run ./cmd/locas-release keygen "$HOME/locasos-publisher-private.b64"
```

The command creates a new file (mode 0600), refuses overwrites, and prints the
corresponding **public** Ed25519 key. Securely back up the private file offline
and record the public key. Do not regenerate it for each release.

## Protect repository settings

In the repository **Settings → Environments**, configure:

- `locasos-releases` with trusted maintainer controls. Add the **secret**
  `LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64` containing the private file's
  single base64 line. Require the appropriate approvals/protections.
- `locasos-stable-promotion` with required reviewers, so promotion is always
  a separate human decision following real Early Access testing.

In **Settings → Secrets and variables → Actions → Variables**, add
`LOCAS_RELEASE_ED25519_PUBLIC_KEY_B64` containing the **public** base64
key printed by the keygen command. This is not secret.

The signed-release workflow rejects a missing private key, a missing pinned
public key, or mismatched keys; it injects only the validated public key into
the updater binary. Development and CI packages remain fail-closed for
privileged remote updates.

## First Early Access release

Only after the signed publisher and protected environments are configured,
the final candidate commit has passed CI, and the updater/rollback tests are
green, create and push a new immutable version tag (example `v0.10.0`).
The tag triggers `Signed LOCAS OS Release`. Before GitHub publishes the
prerelease, the workflow validates the signed manifest against the exact archive
hash using the pinned public key.

An existing **unsigned development appliance** has no pinned publisher key,
and therefore cannot yet execute privileged one-click installs. Its first
migration to a signed release must use the explicit, checksummed manual
upgrade on the trusted Windows machine. Thereafter, the official updater
contains the key and can install future signed releases from the dashboard.

Check the actual Early Access upgrade on TowerDoom, including Windows
localhost health, restart persistence, protected application data and failed
upgrade rollback. Only then manually run the protected `Promote LOCAS Beta to
Stable` workflow with the **same** version tag. Promotion verifies the signed
archive and changes only the prerelease flag; it does not rebuild or replace
release assets.

**No release or promotion is authorized by this document alone.**
