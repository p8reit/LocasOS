# Disposable native QEMU boot harness

This developer harness starts the Debian LocasOS ISO with an **ephemeral qcow2 disk created under a temporary directory**. It does not pass through any physical host block device, enables no network, and removes the scratch image on exit. It is not an installer.

From a Linux development machine with QEMU installed:

```bash
python3 packaging/debian/installer/disposable_vm.py \
  --iso dist/debian/locasos-native-developer-amd64.iso \
  --sha256 EXPECTED_SHA256 \
  --firmware /usr/share/OVMF/OVMF_CODE.fd \
  --duration 120
```

Use `--dry-run` to inspect the exact arguments without launching QEMU. The scratch drive defaults to a sparse 64 GiB virtual qcow2; QEMU runs under TCG so this does not require KVM privileges. The immutable ISO and OVMF code are attached read-only.

**Important limitations:** The command is a time-bounded smoke-test harness, not an automated guest boot verification. A QEMU exit or timeout alone **does not prove** systemd readiness, valid EFI firmware interaction, that services started, or that an operating system has installed to the scratch drive. A future step will add an isolated test-only guest serial readiness signal and an EFI boot assertion.

Automated CI tests cover scratch-disk isolation, bounds, readonly media and checksum helper. Only QEMU-generated virtual media may ever be used for future destructive installer integration tests. Never attach physical disks or allow the native web dashboard to erase drives.
