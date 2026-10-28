# Developer native VM end-to-end install experiment

This phase adds an **experimental, destructive installer for a disposable QEMU virtual disk only**. Nothing here authorizes physical disk installation. The test-only guest script refuses to run unless all of these are present: a LocasOS developer live OS, QEMU DMI product `LocasOS-QEMU-InstallTest`, vendor `QEMU`, a dedicated `opt/locasos/ephemeral` firmware marker with exact value, and a blank >=64 GiB `/dev/vda` virtio block disk with no mounted filesystem, child partition or prior filesystem. It hardcodes the test disk and cannot choose a host device.

The CI job builds the developer Debian ISO and starts two isolated QEMU boots with a temporary sparse qcow2 disk. First boot starts the live Core, emits `LOCASOS_LIVE_READY` to the serial console, and, only when the firmware marker is present, partitions/formats the scratch disk and extracts the offline Debian live squashfs. It installs a removable UEFI GRUB bootloader and a simple root-A boot entry. The second boot removes the ISO and installer firmware marker and waits for `LOCASOS_INSTALLED_READY`, which is only emitted if Core is active. Every test uses `-net none`.

Provisional GPT layout on the scratch disk: EFI 1 GiB, A root 12 GiB, B root 12 GiB (reserved and empty), recovery 4 GiB (reserved and empty), remaining persistent data. This is a developer experiment, **not** functioning A/B updates or recovery.

On CI failure, download the `locasos-native-vm-install-logs` artifact from the workflow run. Never run `install-ephemeral-guest.sh` on a physical machine, expose it via the appliance dashboard, or reuse this test-only command in a customer installer.

Still missing for an actual production native OS: complete A/B boot-slot logic, disk encryption, secure boot, offline recovery, reliable power-loss transactional behavior, signed native package/update chain, supervised physical installer, GPU and hardware readiness. A passing VM acceptance test is necessary but not sufficient for those requirements.
