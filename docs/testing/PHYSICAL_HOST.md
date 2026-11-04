# Physical-host evaluation: safe routes and native installation gates

This guide supports evaluating LocasOS **from** a physical computer. Windows 11
uses the [WSL deployment guide](WSL_DEPLOYMENT.md). A Linux development computer
can collect read-only hardware information and run the Debian developer live ISO
in a diskless QEMU guest. Neither route replaces the physical computer's OS.

**Native installation to a physical disk and writing/booting a physical USB are
not supported by the current product.** The current developer ISO has no
disk installer. Do not extract its application payload into the host filesystem,
adapt VM installer scripts for real disks, or use a WSL archive as boot media.
Keep existing Secure Boot policy: this image has no certified physical Secure
Boot procedure. The [native distribution status](../DEBIAN_NATIVE_DISTRIBUTION.md)
explicitly blocks writing the image to physical machines before its gates pass.

## Choose the test scope

| Computer / objective | Procedure | Evidence it can produce |
| --- | --- | --- |
| Physical Windows 11 x86-64 computer | [WSL deployment](WSL_DEPLOYMENT.md), using an isolated test target | Windows/WSL appliance acceptance; not native installation |
| Physical Linux x86-64 development computer | Inventory and diskless QEMU procedure below | Host inventory, virtual UEFI live boot, guest-local service checks |
| Replace a computer's OS, use physical USB, pass through a real GPU, or retain native appliance data | Stop at the gate checklist below | Planning only; separate implementation and authorization required |

An x86-64 Linux host with an existing graphical desktop, QEMU x86 emulator with
GTK support, and a matching pair of OVMF firmware files is needed below. This
guide does not install or reconfigure host software. If a prerequisite is absent,
record `BLOCKED` and use an appropriately prepared development machine. TCG
emulation needs no KVM access. The example allocates 2 GiB RAM and two virtual
CPUs, following CI; these are test settings, **not validated product minimums**.
Leave sufficient RAM and storage for the host and the downloaded ISO.

## 1. Record host inventory without changing it

Run these in a terminal on the **Linux host**, before launching the guest:

```bash
uname -srmo
lscpu
free -h
lsblk -o NAME,TYPE,SIZE,MODEL,TRAN,MOUNTPOINTS
if command -v lspci >/dev/null; then lspci -nn; fi
if command -v nvidia-smi >/dev/null; then nvidia-smi; fi
```

Record host OS, CPU/RAM, GPU model and existing driver version, plus which
optional tools were absent. Do not install a GPU driver for this procedure.
Review inventory for identifiers before sharing it. Listing a disk is not disk
selection or authorization to modify it. Host GPU enumeration does not prove GPU
visibility or inference inside the guest; this VM has no GPU passthrough.

## 2. Obtain one reviewed development artifact

Use the authentic [LocasOS Actions page](https://github.com/p8reit/LocasOS/actions),
select **Debian Native Live ISO**, and inspect a successful run for the exact
reviewed source revision you intend to test. Record the run URL/ID, attempt,
commit SHA, branch/PR and artifact ID. Check that the workflow definition and
source at that revision match the intended change. A successful run from an
unreviewed PR is not a trusted release.

Download **locasos-debian-native-developer-iso** from that run into a new local
directory and extract it there. Keep these files from the **same artifact**:

- `locasos-native-developer-amd64.iso`
- `locasos-native-developer-amd64.iso.sha256`
- `boot-report.txt`

The workflow currently retains artifacts for **three days**. If the selected
artifact expired or no suitable successful run exists, record the missing run as
`BLOCKED` and request a fresh CI build of the reviewed revision. Do not substitute
an unrelated ISO, an alleged released ISO, or a WSL rootfs. No particular run or
download is certified by this document.

The workflow runs for selected pull-request paths and manual `workflow_dispatch`;
a merge or push alone does not guarantee a new artifact. For a reviewed merged
revision, request a manual run through Actions and verify its resulting commit
SHA before selecting the artifact.

In the host terminal, replace the path with the directory containing the two
matching ISO files, then keep this shell open for the next step:

```bash
set -euo pipefail
artifact_dir='/absolute/path/to/the/extracted/reviewed-artifact'
cd "$artifact_dir"
test -f locasos-native-developer-amd64.iso
test -f locasos-native-developer-amd64.iso.sha256
sha256sum -c locasos-native-developer-amd64.iso.sha256
iso_path="$PWD/locasos-native-developer-amd64.iso"
sha256sum "$iso_path"
```

Stop on a missing file or checksum mismatch; preserve the failure and obtain a
fresh matching artifact. Save the digest in the result record. This compares
bytes with a checksum from the same CI artifact: it detects a mismatch, **does
not authenticate the publisher**, and is not a signed native release manifest.
The build's pinned application update public key also does not sign the ISO.

Inspect `boot-report.txt` for EFI boot metadata and keep it with the run evidence.
Metadata establishes a boot entry, not successful guest boot. The workflow's
35-second diskless QEMU smoke check accepts a still-running emulator at timeout;
it does **not** prove service readiness or complete live boot.

## 3. Boot a diskless UEFI guest

Run this on the **Linux host**, in the shell with the verified `iso_path` above.
Use a local graphical session; the QEMU window provides the guest display and
keyboard. The following searches common Debian/Ubuntu OVMF locations and copies
a fresh matching variables template into a **new temporary directory** on every
launch. Other distributions may use different paths; stop and locate a matching
code/template pair using their package documentation if neither pair exists.

```bash
command -v qemu-system-x86_64 >/dev/null
command -v mktemp >/dev/null
test -f "$iso_path"
ovmf_code=''
ovmf_vars=''
for suffix in _4M ''; do
  code_candidate="/usr/share/OVMF/OVMF_CODE${suffix}.fd"
  vars_candidate="/usr/share/OVMF/OVMF_VARS${suffix}.fd"
  if [[ -f "$code_candidate" && -f "$vars_candidate" ]]; then
    ovmf_code="$code_candidate"
    ovmf_vars="$vars_candidate"
    break
  fi
done
if [[ -z "$ovmf_code" ]]; then
  echo 'BLOCKED: matching OVMF code and variables template required' >&2
  exit 1
fi
vm_session_dir="$(mktemp -d "${TMPDIR:-/tmp}/locasos-live-test.XXXXXXXX")"
cp -- "$ovmf_vars" "$vm_session_dir/OVMF_VARS.fd"
chmod u+w "$vm_session_dir/OVMF_VARS.fd"
printf 'Keep this VM session directory for evidence: %s\n' "$vm_session_dir"
qemu-system-x86_64 \
  -machine q35,accel=tcg -m 2048 -smp 2 \
  -drive "if=pflash,format=raw,readonly=on,file=$ovmf_code" \
  -drive "if=pflash,format=raw,file=$vm_session_dir/OVMF_VARS.fd" \
  -drive "media=cdrom,format=raw,readonly=on,file=$iso_path" \
  -boot order=d -nic none \
  -display gtk -serial "file:$vm_session_dir/guest-serial.log" \
  -monitor none -no-reboot \
  2> "$vm_session_dir/qemu-stderr.log"
```

QEMU writes only session firmware variables and logs on the host. The ISO and
firmware code are read-only; no virtual installation disk is attached. Do not add
host block devices, physical USB/GPU devices, shared directories, writable host
filesystem mounts, or port forwarding. Keep artifact paths free of commas because
QEMU uses commas to separate drive options. If GTK cannot open a local display,
record the error; do not silently substitute a headless invocation that loses
the interactive guest console.

Use the QEMU window to select the normal live boot entry and observe the complete
boot. If a boot/login prompt appears, follow the Debian Live console instructions
for that artifact; this guide does not establish a universal administrator
password. Use the guest's local administrative console for checks that require
privilege, through a trusted local root mechanism provided by the live image.
Do not assume `sudo` is installed or that any default credentials apply. If
console access is unavailable, record it as `BLOCKED` instead of
inventing credentials or changing the image.

This launch intentionally has no networking. Core binds to **guest**
`127.0.0.1:59420`; the host's browser at that address is a different machine.
Ordinary QEMU user-network port forwarding targets a guest network interface and
does not reach a service bound only to guest loopback. Run HTTP checks inside
the guest. There is no SSH server/access assumption and no LAN exposure step.
Remote administration requires a separately reviewed authenticated transport
design; do not change the listener to a wildcard address for this test.

## 4. Check the running live guest

The following commands belong **inside the guest console**, never on the host.
The image build enables `locas-firstboot`, `locas-agent`, `locas-core` and
`locasos-guest-readiness`. First boot sets up restricted service identities and
runtime state. Allow up to 240 seconds for readiness rather than judging a slow
TCG boot by the CI smoke timeout:

```bash
cat /etc/locasos-release
systemctl is-enabled locas-firstboot.service locas-agent.service locas-core.service locasos-guest-readiness.service
systemctl --no-pager status locas-firstboot.service locas-agent.service locas-core.service locasos-guest-readiness.service
```

Expected release edition: `native-developer-preview`. First boot is a oneshot
with `RemainAfterExit=yes`, so `active (exited)` is normal; Core and Agent should
be running. The readiness oneshot can be inactive after successful completion:
inspect its exit result and serial evidence rather than requiring it to remain
active. Poll the public health response with a bounded loop in a guest Bash
shell:

```bash
set -euo pipefail
guest_health_ok=0
for attempt in {1..120}; do
  if curl --fail --silent --show-error --max-time 3 --noproxy '*' \
      http://127.0.0.1:59420/healthz | \
      python3 -c 'import json,sys; h=json.load(sys.stdin); sys.exit(0 if isinstance(h,dict) and h.get("component")=="locas-core" and h.get("status")=="ok" else 1)'; then
    guest_health_ok=1
    break
  fi
  sleep 2
done
test "$guest_health_ok" -eq 1
curl --fail --silent --show-error --max-time 5 --noproxy '*' http://127.0.0.1:59420/healthz
ss -ltn '( sport = :59420 )'
curl --silent --show-error --max-time 5 --noproxy '*' \
  -o /dev/null -w '%{http_code}\n' http://127.0.0.1:59420/api/telemetry
```

Expect exact Core health fields, a listener at `127.0.0.1:59420` with no wildcard
or external listener for that port, and HTTP **401** for unauthenticated
`/api/telemetry`. Polling is bounded to 120 attempts (up to about ten minutes
including HTTP timeouts); the readiness service itself defaults to 240 seconds.
Do not label an HTTP response alone as complete hardware or installation proof.

For read-only guest telemetry diagnostics, run the following from a **root
console inside this disposable guest**. `runuser` uses the Core's existing
identity/group required by the agent's Unix-socket peer check; it does not change
account permissions or enable network access:

```bash
runuser -u locas-core -g locas-agent -- \
  curl --fail --silent --show-error --max-time 5 --noproxy '*' \
  --unix-socket /run/locas/agent.sock http://agent/internal/telemetry
systemctl show locasos-guest-readiness.service -p ActiveState -p Result -p ExecMainStatus
journalctl -b -u locas-firstboot.service -u locas-agent.service -u locas-core.service \
  -u locasos-guest-readiness.service --no-pager -n 100
```

Record CPU and memory as **virtual guest** telemetry. GPU unavailable/empty is
expected without passthrough and is not a host GPU failure. The native ISO
provides no validated NVIDIA driver/CUDA stack, enrolled administrator, downloaded
model or demonstrated inference. The serial log should contain
`LOCASOS_GUEST_HEALTH_OK` for the readiness service's narrower systemd/Core health
assertion; a failure marker or missing marker needs investigation alongside the
console, unit results and logs. This marker is not installation evidence.

Confirm the native worker paths remain disabled and inactive in this fresh live guest. The state checks below are read-only; a mismatch is a failed gate, not an instruction to enable or repair them:

```bash
for path_unit in locas-app-worker.path locas-update-worker.path; do
  path_enabled="$(systemctl is-enabled "$path_unit" 2>/dev/null || true)"
  path_active="$(systemctl is-active "$path_unit" 2>/dev/null || true)"
  printf '%s enabled=%s active=%s\n' "$path_unit" "$path_enabled" "$path_active"
  test "$path_enabled" = disabled
  test "$path_active" = inactive
done
```

Authenticated dashboard enrollment/login/logout and protected HTTP telemetry are
separate acceptance items; this console procedure does not claim them passed.
Do not enable the bundled app/update worker paths or apply WSL updates to fill
these gaps: native app/OS lifecycle approval and recovery remain incomplete.

## 5. End the session and preserve evidence

At the **guest** administrative console, use `systemctl poweroff` and wait for
QEMU to exit. Do not issue a host poweroff command. If the guest is stuck, close
only its QEMU window and record an abnormal stop. Keep the verified ISO,
checksum, boot report, firmware paths, session directory, console observations
and relevant logs until the test is reviewed. Review logs for secrets before
sharing. This procedure deliberately gives no recursive cleanup command.

All guest filesystem changes are ephemeral; the next launch is a fresh live
session with fresh firmware variables. No persistence, reboot installation,
native update/rollback or recovery result follows from a successful live boot.
For existing VM harness scope and limitations see
[Disposable native QEMU harness](../NATIVE_DISPOSABLE_VM_HARNESS.md).

## Physical installation remains gated

Before anyone supplies a physical installation/USB procedure, A1 Platform and A6
Security/QA must provide reviewable evidence and obtain the required human
authorization for the real target. The binding
[installer architecture](../NATIVE_INSTALLER_ARCHITECTURE.md) requires:

- Reproducible/pinned image inputs, authenticated native payload/packages and
  independently verified publisher-signed native manifests; build provenance,
  SBOM, licenses and redistribution review.
- Full UEFI VM boot and installation into guest-only disposable virtual disks,
  exact target identity/topology checks at write time, persistent-data acceptance
  across independent boots, and runtime isolation beyond source-policy checks.
- A dedicated locally controlled physical installer with non-destructive default,
  displayed stable disk identity/model/serial/size/partitions, typed target and
  destructive confirmation, and rejection of installer/system/mounted disks.
- Implemented partition layout, persistent application/model/admin data,
  enrollment, signed OS updates, boot-failure handling, recovery and rollback;
  fault injection, offline rollback and backup/restore evidence.
- A defined Secure Boot trust/enrollment/revocation design tested on real hardware;
  existing production firmware policy is not to be relaxed by this guide.
- A separately approved physical USB boot safety plan, acceptance on at least two
  distinct Intel/AMD UEFI platforms, actual NVIDIA driver/CUDA/Ollama inference
  tests, and observed minimum resource requirements.
- Headless operation plus Ethernet/offline/DNS failure, reboot, power-loss and
  clock-skew acceptance, with no unapproved network listener exposure.
- Explicit human approval of the physical target and disk operation, and separate
  release promotion approval after all applicable gates pass. A green source CI
  run or the VM prototype allowlist grants neither approval.

Real GPU passthrough and durable native installation require separate test plans;
they are not extensions to the diskless command above.

## Record results without overstating coverage

Use [TEST_DEPLOYMENTS](../TEST_DEPLOYMENTS.md) for the overall matrix and
[ACCEPTANCE_RESULTS](ACCEPTANCE_RESULTS.md) as the primary result record. Enter
`PASS`, `FAIL`, `BLOCKED` or `NOT RUN` for each test, with these fields:

| Field | Required detail |
| --- | --- |
| Source/artifact | Reviewed commit, run URL/ID/attempt, artifact ID, ISO SHA-256, checksum result; development trust limitation |
| Host/VM | Host OS/CPU/RAM/GPU, QEMU version, OVMF code/template paths, command and allocated resources |
| Boot | EFI metadata, observed live console boot, timestamps, session log paths and readiness marker/result |
| Services/API | Release identity, unit states, exact health fields, loopback listener and unauthenticated 401 |
| Telemetry/auth | Virtual CPU/RAM result; GPU scope; enrollment/login/logout and authenticated HTTP checks individually marked |
| Session end | Guest shutdown result; abnormal termination if any; evidence retained |
| Open gates | Physical install/USB, GPU inference, persistence, updates, recovery and hardware acceptance remain unproved |

Do not mark hardware/WSL/UEFI checks passed because this document's commands were
reviewed. Execution evidence must come from the stated host and exact artifact.
