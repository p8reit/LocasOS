# LOS-108: Native installer safety and release gates

Status: initial threat-model skeleton; full security analysis and automated gating pending. Owner D; draft PR required.

## Assets / threats
Protect physical host disks, USB install media, mounted live root, preexisting file systems, LVM/RAID/multipath assemblies, UEFI boot entries, model/customer data, firmware variables, and signature trust roots. Attacker or mistake may provide fake /dev/disk/by-id links, multiple devices, stale enumerations, spoofed QEMU/DMI/fw_cfg markers, malformed image metadata or symlink swap after preflight.

## Current hard boundary
The only destructive code allowed in this milestone is a test-only module compiled/packaged solely for the disposable QEMU guest. No host /dev passthrough, no physical installation code exposed, no network/dashboard wipe endpoint, no automatic install on normal boot. Require fail-closed identity checks and single-device matching immediately before disk writes. A VM marker is not proof of physical-device safety.

## Negative acceptance
Bad identity, mounted or existing data, zero/multiple disks, disallowed transport, missing OVMF variables, incomplete signature, corrupt ISO, no explicit opt-in, time-of-check/time-of-use mutation, power interruption: must not produce false success. Test fixture safety and CI static checks are blocking.

## Not implemented
Actual boot slot switching, transactional rollback, encrypted persistent data, Secure Boot, signed update rollout, real USB boot and hardware compatibility remain separate release gates requiring operator approval.
