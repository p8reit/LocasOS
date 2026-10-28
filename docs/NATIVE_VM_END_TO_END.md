# Native developer VM installer — disposable QEMU only

This is the first integrated native VM installation experiment. It is **not** a physical disk installer and must not be released as one.

## Acceptance path

The Debian developer Live ISO includes an optional developer-only oneshot service. It is inert unless the guest sees an exact `opt/locasos/dev-install` QEMU fw_cfg opt-in marker. The installer rechecks that it is a LocasOS developer Live session running under QEMU and finds exactly the expected blank virtio-scsi disk by synthetic serial `locasos-scratch`. It rejects existing partitions, recognized filesystems, mounts, removable/readonly storage and disks outside its 64–128 GiB envelope.

Inside that explicitly opted-in virtual guest, the experimental installer creates an EFI system partition, OS A and B, recovery and persistent data, copies the live root into OS A, writes fstab, installs a removable-path EFI GRUB loader and emits a serial completion marker.

CI then **shuts down that VM** and boots a second VM using the scratch disk alone, requiring a new serial health marker from active Core and Agent. The scratch image is deleted after the test.

The readiness signal `LOCASOS_GUEST_HEALTH_OK` means systemd sees both services active and the guest loopback health endpoint succeeds. A QEMU process remaining alive is explicitly not sufficient.

## Constraints and remaining product work

- A/B layout is initialized, but boot slot switching, signed OS updates, recovery boot and rollback are not yet built.
- Bootability and persistence are proven only if the dedicated CI integration job succeeds; do not infer success from script presence or unit tests.
- No disk install or erase button in the web dashboard. No generalized physical-device installer interface exists.
- Native ISO and application artifact signatures, Secure Boot, user data encryption, hardware drivers and clean-console installer UI still need independent development and validation.
- The test uses ephemeral CI storage. Never replace the QEMU scratch disk with a physical /dev node, mount a real host volume inside QEMU or copy this test-only installer into Stable images.

## Manual development test

On a Linux QEMU development host, build the ISO using `packaging/debian/build-live-iso.sh`. Then use:

```bash
python3 packaging/debian/installer/verify_native_vm_install.py \
 --iso dist/debian/locasos-native-developer-amd64.iso \
 --sha256 "$(cut -d ' ' -f 1 dist/debian/locasos-native-developer-amd64.iso.sha256)" \
 --ovmf-code /usr/share/OVMF/OVMF_CODE_4M.fd \
 --ovmf-vars /usr/share/OVMF/OVMF_VARS_4M.fd
```

QEMU runs with TCG (no KVM access required), no network and a newly created temporary qcow2. If any install, boot, or health assertion fails, the test exits unsuccessfully and deletes the temporary image.

## Integration experiment — October 28, 2026

This **unmerged** integration branch now trials three independent workstream components: developer-only guest readiness diagnostics from #110; exact QEMU virtual target preflight and a persistent-data sentinel from #111; and stage-local serial evidence parsing from #112. These are experimental changes to find the actual D-Bus/Agent/Core boot fault, not proof that the native installation works. The live-stage timeout is extended for GitHub TCG performance; acceptance criteria remain unchanged. The physical disk installer is still unavailable.
