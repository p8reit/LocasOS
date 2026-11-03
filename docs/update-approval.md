# Signed artifact-bound dashboard update approval

Phase P09E / issue [LOS-120](https://github.com/p8reit/LocasOS/issues/120). Implementation scope: authenticated WSL management API through the independent root update worker. This is not a production release or a native OS installer/update contract.

## User flow and trust boundaries

1. An authenticated administrator saves Stable or Early Access. This is an appliance-wide preference; saving it invalidates every session's pending discovery and selection.
2. Check releases. Core uses the fixed GitHub repository/tag namespace, validates metadata sizes and unique asset names, then verifies the detached Ed25519 signature against the **original manifest bytes** with its build-time pinned publisher public key. It validates the manifest platform/file/version and reads its archive SHA-256. No key from the release or browser is trusted.
3. Core returns a random 256-bit token bound to that session, exact version/channel/digest, and a ten-minute expiry. The dashboard displays the digest and includes it in the installation confirmation. A package link is not an install authorization. Developer builds without a pinned key may show manual assets but cannot issue dashboard approvals.
4. Install consumes the matching selection atomically before making **one** agent enqueue attempt. Consumption happens even if IPC fails or reports a queue conflict: the administrator must check releases again before retrying. Logout, recheck, preference changes, expiry and core restart invalidate the selection. Late HTTP responses cannot recreate invalidated authorization. Core caps outstanding session selections at 256.
5. The peer-authorized agent validates the typed job, bounds/strictly decodes its JSON, synchronizes it to disk and creates the request without overwriting an existing job. It preserves approved version/channel/digest, token and expiry. The token is an opaque job identity, not independent root authorization.
6. Root consumes the queued request once and validates its lifetime at consumption. It independently fetches and verifies signed metadata with its own pinned key. Both the signed digest and actual downloaded archive must match the approved SHA-256 **before** invoking installation. A different validly signed artifact under the same version is rejected. Existing channel/downgrade, archive and OS-base checks, health verification and recovery remain required. An already-started download can finish after selection expiry; expiry bounds authorization to start, not job execution time.

Approvals are deliberately in memory in Core and single-use at the authenticated enqueue boundary. This is not a durable root replay journal, a package audit ledger or hard protection against a compromised privileged/agent identity. P09F now blocks known same-version/no-op dashboard installation as well as older versions. A same-version repair requires an explicitly verified manual recovery operation; see [version policy](update-version-policy.md). Unknown legacy versions retain a clearly warned signed replacement path.

## Runtime API contract

All endpoints require the administrator cookie. Mutations require same-origin JSON plus `X-Locas-Request: 1`. Responses use `Cache-Control: no-store`. The runtime contract here is separate from the older proposed `/api/v1/*` scaffold.

`GET /api/updates/check?channel=stable` keeps `package_ready`, `package_files`, `latest_release_tag`, `release_url` and `channel`. It adds:

```json
{
  "install_ready": true,
  "approval": {
    "version": "v1.2.3",
    "channel": "stable",
    "sha256": "<64 lowercase hexadecimal characters>",
    "token": "<64 lowercase hexadecimal characters>"
  },
  "approval_expires_at": "<RFC3339 timestamp>"
}
```

When metadata is incomplete, the key is unavailable, signature verification fails, authentication changes or discovery is invalidated, `install_ready` remains false and no approval is issued. Invalid asset namespace, duplicate assets or release identity return 502; unavailable discovery returns 503. No update is queued by a check.

`POST /api/updates/install` accepts **exactly the four approval fields above**. Core rejects unknown fields, malformed values and trailing JSON (400), mismatched/expired/consumed selection or saved channel (409), unauthenticated access (401), and unsafe mutations (403). It forwards a validated job to restricted IPC and returns the enqueue outcome. Clients must not add URLs, shell commands, trusted keys or their own expiry.

`POST /api/updates/channel` accepts `{"channel":"stable"}` or `{"channel":"beta"}`; `beta` is the existing internal Early Access identifier. `GET /api/updates` and `/api/updates/status` retain their existing version/preference/status behavior.

## Packaging and compatibility

Official WSL/native payload build scripts pin the same configured public key into Core and updater using linker flags; the private signing key is never included. Trust configuration is unchanged. Native developer worker paths stay disabled; pinning does not make that ISO a production updater.

The installer lock is root-owned at `/var/lib/locas-upgrades/upgrade.lock`, under an existing update-worker `ReadWritePaths` directory. It no longer requires write access to `/run` under `ProtectSystem=strict`. Recovery backups share that protected directory.

Version/channel-only legacy installation requests now fail closed. Deploy Core, agent, updater, embedded UI and units from the same coherent bundle and restart the services together. Old pending requests are rejected; perform fresh discovery after upgrading. No unsafe compatibility fallback is introduced. Manual verified-bundle recovery remains available.

## Validation limits

Source-level API, store concurrency, queue, worker substitution and embedded JavaScript tests cover this contract. Filesystem-isolated upgrade tests exercise read-only `/run` and lock contention in addition to prior rollback/safety cases. These do not prove a real Windows/WSL installation, production signing, interrupted power-loss recovery or real-GPU inference. Full target qualification and independent release approval remain separate gates.

## P09F identity and eligibility

Discovery now reports running build/commit separately from the last verified update record and revalidates numeric eligibility before issuing/consuming an approval. `install_eligible` is independent of signature/package readiness; a blocked candidate receives no approval. The exact four-field installation request remains unchanged. See [build identity and version policy](update-version-policy.md).
