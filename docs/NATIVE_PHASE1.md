# Native LocasOS phase 1 — executable VM foundation

**Scope:** A VM-only native Ubuntu Server 24.04 prototype. This is *not* a bootable LocasOS ISO, is *not* a disk installer, and is *not* approved for physical disks. The established WSL Stable release remains a separate build and update stream.

## Architecture

- Source of truth: Go LocasOS Core, Agent, and Updater, and the shared systemd service definitions.
- Native packaging: `packaging/native/build-payload.sh` stages Linux binaries and service units in a checksummed tar archive.
- VM provisioning: `packaging/native/install-native-vm.sh` installs the payload into an **already provisioned disposable Ubuntu Server 24.04 guest**. It will not partition or format disks, change bootloader configuration, enable public HTTP listeners, or install host GPU drivers.
- The WSL-specific `wsl.conf`, keepalive scripts, and WSL importer are deliberately excluded.
- Only loopback access to port 59420 is supported in phase 1. Access through a hypervisor console, guest-local browser/SSH port forwarding, or a deliberately scoped development tunnel; do not expose plaintext HTTP on a LAN.
- Ubuntu service accounts `locas-core`, `locas-agent`, and `locas-ollama` have no login shell. Privileged requests continue to use the preexisting Unix socket allowlist.

## Build payload in Ubuntu CI / development

```bash
export LOCAS_UPDATE_PUBLIC_KEY_B64="<trusted release public key>"
bash packaging/native/build-payload.sh
bash packaging/native/validate-payload.sh
```

The updater build deliberately rejects an absent pinned signing key. A matching SHA-256 protects accidental corruption and is **not publisher authentication**. Never treat this prototype tarball as a signed release.

## Create an isolated UEFI VM on an Ubuntu Linux host

Install `qemu-system-x86`, `qemu-utils`, and `ovmf` on a **Linux development host**. Independently verify the Ubuntu Server 24.04 amd64 ISO before use. The launcher creates **only** a new 40 GiB virtual qcow2 disk and disposable OVMF variables in an empty workspace; no host block devices are passed through:

```bash
bash packaging/native/launch-uefi-vm.sh ~/Downloads/ubuntu-24.04-live-server-amd64.iso ~/locasos-native-vm-test
# Complete interactive Ubuntu Server installation on the VM's virtual disk.
# Shut down the guest, then boot its virtual disk without the installer ISO:
bash packaging/native/launch-uefi-vm.sh --resume ~/locasos-native-vm-test
```

The ISO may have a point-release suffix; the launcher deliberately accepts only the `ubuntu-24.04*live-server-amd64.iso` naming family. ISO filename matching **does not prove authenticity**; verify its upstream signature/checksum independently. If a real boot attempt fails, preserve the virtual disk and VM logs; never retry against physical drives.

## Use only in a disposable Ubuntu Server 24.04 VM

Copy both files from `dist/native/` into the VM, along with `install-native-vm.sh`. On the guest:

```bash
sudo env LOCAS_NATIVE_VM_ACK=I_UNDERSTAND_THIS_IS_A_DISPOSABLE_VM \
  bash ./install-native-vm.sh ./locasos-native-payload-amd64.tar
systemctl is-active locas-agent.service locas-core.service
curl -fsS http://127.0.0.1:59420/healthz
sudo bash ./test-native-vm.sh
sudo systemctl reboot
# After reboot repeat: sudo bash ./test-native-vm.sh
```

The script refuses WSL, non-Ubuntu-24.04 guests, non-systemd environments, missing operator acknowledgment, invalid checksums, and tar archives with unexpected top-level members. The guest must have `curl`, `tar`, and `systemd`. A VM snapshot is required before exercising any prototype installer.

## Acceptance gates

1. CI shell lint, Go tests, reproducible staging content inventory, and checksum verification.
2. Disposable x86-64 UEFI VM boots a clean Ubuntu Server 24.04 installation.
3. Native payload provisions firstboot, agent, core, path workers, and responds with `{"component":"locas-core","status":"ok"}` within the guest.
4. Reboot maintains service health and protects administrator enrollment.
5. A negative test proves WSL and physical hosts and non-VM environments reject installation.
6. Later work: reproducible **bootable ISO** using official Ubuntu installer with independent, authenticated base-image verification; safe human-confirmed disk selection; offline install and recovery media; GPU/CUDA hardware qualification; signed native release, A/B system rollback, power-loss tests.

**Hard stop:** No unattended destructive partitioning, no physical-disk support, no production native release, and no unsigned ISO claims until dedicated verification and recovery milestones pass.
