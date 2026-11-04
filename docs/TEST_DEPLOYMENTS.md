# Test deployment guides

Use these guides to test a reviewed LocasOS build without replacing an existing appliance. They describe current source behavior at `a91e9e2` (P09G/P10A merged), not a new published release or a claim that your hardware has passed.

**Start with WSL on a spare Windows 11 machine or a clean Windows VM.** If you mean installing LocasOS directly onto a physical PC's SSD, that installer is not available yet. The native developer ISO has no disk installer and is not approved physical boot media. The physical-host guide provides read-only readiness checks and a diskless VM test on your Linux computer, plus the gates still required for native installation.

## Choose your path

| Goal | Guide | What you can establish |
|---|---|---|
| Run the appliance on physical Windows 11 hardware or in a prepared Windows VM | [WSL deployment](testing/WSL_DEPLOYMENT.md) | Import, first boot, browser setup/authentication, current app/runtime controls, audit journals and Windows lifecycle |
| Evaluate the Debian native image from a physical Linux computer | [Physical-host readiness and diskless VM](testing/PHYSICAL_HOST.md) | Host inventory, reviewed ISO bytes, virtual UEFI live boot and guest-local service health; no host disk installation |
| Record results, evidence and open blockers | [Acceptance record](testing/ACCEPTANCE_RESULTS.md) | Comparable PASS/FAIL/BLOCKED/NOT RUN results tied to exact source, artifact and machine |
| Qualify a published signed WSL release | [Release deployment guide](DEPLOYMENT_GUIDE.md) / [WSL release installation](WSL_INSTALLATION.md) | Publisher-verified release path; the selected release may predate the current merged source |
| Test P10A queue logic | Linux developer checkout section in the [WSL guide](testing/WSL_DEPLOYMENT.md#test-p10a-in-a-linux-developer-checkout) | Automated metadata persistence/admission tests; no live GPU job dispatch |

Keep the download workspace, registered WSL filesystem, developer checkout, VM session directory and backup destination separate. The standard development WSL installer supports only `LocasOS` on port 59420. It cannot create an arbitrary side-by-side test name. Use a clean target; the separate signed Debian release helper has its own constraints. A different localhost port does not isolate browser cookies: use a dedicated browser profile for each appliance.

## Obtain the right build

A merge updates source, not an installed appliance or an already published release. To test P09G/P10A, choose a successful reviewed workflow run whose source contains those merges. Record the full source revision, workflow/run/attempt, artifact identity and file digest before import or boot. Use the step-by-step provenance/download checks in the selected guide.

| Workflow | Artifact | Current contents / retention | Trust boundary |
|---|---|---|---|
| WSL Appliance Build (`wsl-appliance.yml`) | `locasos-wsl-dev-rootfs` | Complete development rootfs, checksum, build info and Windows helpers; 7 days | Reviewed GitHub Actions development artifact, not a publisher-signed release |
| Debian Native Live ISO (`debian-native-iso.yml`) | `locasos-debian-native-developer-iso` | Developer ISO, checksum, EFI metadata report; 3 days | CI-only developer image; checksum verifies consistency, not publisher identity or boot health |
| General CI (`ci.yml`) | `locasos-verified-upgrade` | Upgrade tar/scripts/checksum; 7 days | Not a fresh-install rootfs or native OS image; do not use it as one |

Do not mix helpers, archives or checksums from different builds. Downloading a whole artifact from the authentic reviewed run matters; a checksum supplied alongside arbitrary bytes is not independent authenticity proof. Artifacts expire. If missing, select or request a fresh qualifying run at the reviewed revision; do not replace it with an unrelated cached image. WSL supports `main` push builds and manual dispatch; the Debian ISO workflow is PR-path/manual-dispatch only, so a merge alone does not generate a new main ISO.

For PR builds, the PR head SHA and the synthetic merge SHA used by checkout/`GITHUB_SHA` can differ. Record head/base/merge evidence rather than forcing one value to match the other. Prefer a matching `main` WSL build for merged-source acceptance. The guides use operator-selected inputs rather than fixed run IDs that will expire.

## Suggested first test session

1. Select one reviewed build and fill the acceptance record's identity section. Confirm an empty disposable target and sufficient storage; keep existing production installations intact.
2. Follow one deployment guide through health and anonymous-access checks. Record exact JSON identity, service states and any missing prerequisites. A green service or HTTP 200 alone is insufficient.
3. For WSL, enroll/login using a dedicated browser profile and a unique password; test only the optional downloads/actions you choose to authorize. Verify the P09G journals under their service-owner UIDs.
4. Test console-close and targeted restart behavior, then separately schedule sign-in/reboot checks on the test machine. Native diskless guest state is ephemeral; do not expect native persistence or enabled app/update workers.
5. Run P10A source tests in a separate prepared Linux/WSL developer checkout if desired. The appliance does not ship the Go SDK and does not currently expose a job queue UI/API.
6. Preserve private evidence and quiescent backups. Reconcile pending/uncertain operations before retrying. Stop at unsupported install/recovery gates rather than changing trust settings or deleting state to get a pass.

## What this testing does not establish

P10A stores metadata/reservations only; it does not stop an orphan GPU process or execute queued payloads. See [queue contract](architecture/compute-queue.md). P09G local chains cannot defeat root/writer history replacement without independent anchors; see [audit operations](security/auditing.md). A healthy development appliance is not proof of signed update qualification, actual GPU inference, physical installation, power-loss recovery or product release readiness.

No procedure here publishes a release, replaces the host OS, changes production trust keys, erases a disk, registers physical boot media, or automatically unregisters a WSL distro. Those actions require their own implementation/readiness and explicit operator decisions. The guides were source-reviewed; actual Windows/WSL/hardware results must come from your recorded execution.
