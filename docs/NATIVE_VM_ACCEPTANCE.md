# Phase 2 integration acceptance — disposable UEFI autoinstall

**Destructive INSIDE A NEW QEMU VIRTUAL DISK ONLY.** This suite never attaches a host physical block device. Do NOT transplant the generated NoCloud seed to physical installer media: it contains unattended storage instructions and is deliberately limited by both QEMU identity and the exact uniquely serial-numbered test disk. Do not enable on native hardware.

Canonical Ubuntu documents NoCloud autoinstall using an external `CIDATA` volume and `autoinstall` kernel argument. The LocasOS VM integration harness uses that technique, with stricter disk identity and a required operator acknowledgment. It **does not change the official experimental ISO** to unattended mode.

## Requirements

A Linux x86-64 development machine with QEMU, `qemu-img`, OVMF 4M, `cloud-image-utils`, `xorriso`, `python3`, `openssl`, `ssh`. Acquire and authenticate the Ubuntu Server 24.04.5 image using `verify-ubuntu-iso.sh` and the independently trusted Canonical CD signing key. Work only with an empty disposable workspace and an ephemeral testing SSH public key.

```bash
ssh-keygen -t ed25519 -N '' -f /tmp/locasos-vm-acceptance-key
export LOCAS_NATIVE_VM_AUTOINSTALL_ACK=I_ACCEPT_ERASING_A_DISPOSABLE_QEMU_IMAGE_ONLY

bash packaging/native/make-vm-autoinstall-seed.sh \
    ~/locas-vm-seed /tmp/locasos-vm-acceptance-key.pub

bash packaging/native/launch-vm-autoinstall.sh \
    ~/isos/ubuntu-24.04.5-live-server-amd64.iso \
    ~/locas-vm-seed/seed.iso \
    ~/locas-vm-disposable
```

The launcher creates ONLY `~/locas-vm-disposable/disposable.qcow2` (40 GiB virtual capacity) and OVMF variables inside that new directory. It extracts the authenticated ISO's kernel/initrd and uses QEMU direct Linux boot with the NoCloud seed. The guest autoinstall early commands refuse non-QEMU systems, refuse multiple disks, and refuse anything but virtual disk serial `LOCASOS_VM_ONLY_001`. Its Ubuntu storage layout also targets that exact serial. If any guard fails, installation MUST halt; do not weaken guards.

Once installation finishes and QEMU powers off:

```bash
bash packaging/native/boot-installed-vm.sh ~/locas-vm-disposable
# Separate terminal; only host loopback port 2222 is forwarded to guest SSH:
ssh -i /tmp/locasos-vm-acceptance-key -p 2222 \
    -o StrictHostKeyChecking=accept-new locasdev@127.0.0.1
```

With a successful guest login, obtain a native signed payload and verifier through reviewed/trusted source paths, not untrusted guest-local executables. Copy the archive, `.sha256`, `locasos-native.json`, `locasos-native.sig`, `install-native-vm.sh`, `verify-archive.py`, `test-native-vm.sh`, and the independently trusted `locas-release` binary to a working directory in the VM. Require the separately authenticated public key in the VM environment:

```bash
export LOCAS_UPDATE_PUBLIC_KEY_B64='<independently validated publisher key>'
export LOCAS_NATIVE_VERIFY_BIN=/path/to/trusted/locas-release
export LOCAS_NATIVE_VERSION=vX.Y.Z
sudo --preserve-env=LOCAS_UPDATE_PUBLIC_KEY_B64,LOCAS_NATIVE_VERIFY_BIN,LOCAS_NATIVE_VERSION \
    env LOCAS_NATIVE_VM_ACK=I_UNDERSTAND_THIS_IS_A_DISPOSABLE_VM \
    bash ./install-native-vm.sh ./locasos-native-payload-amd64.tar

sudo bash ./test-native-vm.sh
sudo reboot
# Reconnect and repeat test-native-vm.sh.
```

**Not yet an end-to-end automated acceptance pass:** Success requires an observed Ubuntu VM install, guest SSH login, signed native application deployment, actual `/healthz`, and reboot persistence. Do not infer any of those from CI schema or source checks alone. The untrusted experimental ISO must never be promoted to a signed native release on this evidence alone.

See `docs/NATIVE_PHASE2.md` and issues #81–#84 for ISO trust, OS recovery, GPU and stable-release criteria.
