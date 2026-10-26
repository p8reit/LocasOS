# Debian 13 parity: LocasOS WSL and native

LocasOS WSL Early Access is transitioning from Ubuntu 24.04 to **Debian 13 (Trixie)**, matching the Debian-based LocasOS native live image. Both targets reuse the same Core/Agent executables, firstboot scripts, service units and `packaging/common/base-packages.txt` base dependency set. The WSL image still includes WSL-only files such as `/etc/wsl.conf`, imported through the signed Windows WSL bootstrap; the native live image is not imported into WSL.

## Compatibility

The existing **v0.10.6 Stable WSL release remains Ubuntu 24.04** and is not modified. Moving between Ubuntu and Debian is an OS distribution migration, **not an application update**. Do not apply a Debian WSL rootfs or upgrade bundle to an installed Ubuntu LocasOS instance; preservation of models, Docker volumes, credentials and administrator data across distros requires a separate export/import design and testing.

For Early Access testing use an empty WSL distro registration on a disposable host. The installer should fail closed on occupied LocasOS registrations; obtain a backup with `wsl --export` before any manual reset. Do not unregister a production WSL instance to test this change.

## Acceptance checklist

1. CI builds `locasos-wsl-amd64.tar` with `PRETTY_NAME` from Debian 13; inspect `/etc/os-release` in the Docker image and exported archive.
2. Rootfs includes `systemd`, PAM user-session and D-Bus dependencies for WSL's systemd login.
3. Core/Agent and firstboot systemd unit activation is unchanged, dashboard still listens only on `127.0.0.1:59420`.
4. Native live image and WSL rootfs share the base packages manifest; CI checks the references.
5. Existing signed upgrade archive remains an **application upgrade only** until distro migration protections are implemented.
6. Test WSL `--import` on a clean Windows VM; no root-session warning; healthz returns OK; restart, Windows sign-in and admin enrollment succeed.
7. Publish a new signed **Early Access** tag only after the importer and real Windows validation pass. Do not relabel v0.10.6 or claim Debian Stable without those gates.

See `docs/DEBIAN_NATIVE_DISTRIBUTION.md` for the independently built native live image.
