# Developer VM disk installation

This phase adds Debian's interactive live-image installer to the LocasOS ISO.
It is a developer VM installation path, separate from the signed WSL release.
Physical installation, production signing, A/B upgrades and recovery remain
unsupported. The earlier live-only ISO cannot acquire an installer at runtime;
use the new ISO from the exact reviewed build.

## Prepare Hyper-V

Use a Generation 2 VM with at least 4 GiB RAM and a **new, empty virtual disk**
of at least 64 GiB. Attach only that virtual disk and the ISO. Never attach a
host disk, pass-through disk, shared production disk or any disk containing data
you need. On the Windows host, inspect attachments before starting:

```powershell
Get-VMHardDiskDrive -VMName 'LocasOS' | Format-List Path,DiskNumber
```

Every hard disk must have a `.vhdx` file path and no physical `DiskNumber`.
Check the selected file with `Get-VHD -Path '<the path shown above>'`.
The installer checks UEFI and known Hyper-V/QEMU DMI identities. That check
does **not** prove that a disk is a VHDX or exclude host pass-through: the host
attachment check is an operator responsibility. Do not use this image outside
the documented disposable VM configuration.

Keep the Secure Boot setting that successfully booted the developer live image;
this phase does not establish a new Secure Boot trust chain.

## Install

1. Boot the new ISO and select **Install** in its boot menu. The live entry still
   starts an ephemeral session and does not install anything.
2. Follow Debian's language, keyboard, hostname and networking prompts.
3. Create your own administrator account and password. There is no installed
   shared `user/live` account or embedded administrator password.
4. Select the new virtual disk. Review its displayed model and size. For a new
   VM, guided partitioning with all files in one partition is the simplest
   option. Disk selection, partitioning and the final write confirmation remain
   interactive; confirming them erases the selected virtual disk.
5. Complete bootloader installation. Shut down, eject the ISO in Hyper-V, and
   start the VM from its virtual disk.

The installer copies the immutable live image using Debian live-installer;
it does not copy the running live session or any dashboard enrollment performed
there. Debian configures the root filesystem, ESP, bootloader and selected user.
The completion hook resets machine identity, generates new SSH host keys,
enables SSH and LocasOS services, and regenerates installed initramfs.

## Access and persistence checks

Log in with the account created during installation. Find the VM's address with
`hostname -I`, then on Windows run:

```powershell
ssh -N -L 59421:127.0.0.1:59420 <your-user>@<vm-address>
```

Open `http://127.0.0.1:59421/` and complete administrator enrollment. The dashboard
continues to bind to guest loopback; it is not exposed directly on the LAN.
SSH starts on the installed system with your selected credentials. Live-session
SSH remains opt-in and does not inherit a known live password onto disk.

Inside the installed VM verify:

```bash
systemctl is-active dbus ssh locas-agent locas-core
curl --fail http://127.0.0.1:59420/healthz
findmnt /
```

The root filesystem must be the virtual disk, not a live overlay. Create a small
file in your home directory, reboot again with the ISO still detached, and
verify that the file, account and dashboard enrollment persist.

## Automated acceptance

The ISO workflow verifies the actual built installer initrd and runs an isolated
QEMU/OVMF installation onto a newly created qcow2 file. Test-only unattended
preseeding is injected into a temporary initrd copy; it is not shipped in the
ISO. It requires the QEMU firmware opt-in and the exact synthetic scratch disk
identity before any partition confirmation. No host disk is mapped to QEMU.
The test then boots the installed disk twice without the ISO, requiring fresh
service/API and persistence markers on each boot. Uptime alone is insufficient.

CI QEMU results do not certify Hyper-V operation. Record the exact ISO checksum,
Hyper-V VM settings, disk-only boots and persistence results during user testing.
