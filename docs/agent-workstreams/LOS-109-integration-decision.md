# LOS-109: Native QEMU installer integration decision

Status: **decision proposed, not merged**. The overlapping PRs #102 and #103 both touch ISO builder and workflow while adding distinct test-only guest installer and QEMU runner code. Merge neither directly until reviewed.

## Candidate comparison
| Criterion | PR #102 | PR #103 |
|---|---|---|
| Live ISO build | Succeeded in observed CI | Succeeded in observed CI |
| Full acceptance | Failed: 120-second live/readiness/installer markers missing | Failed: dbus.service and dependent Agent/Core readiness; guest health marker missing |
| Installer interface | Hardcodes /dev/vda virtio-blk; QEMU DMI/fw_cfg marker | QEMU marker plus scsi by-id serial locasos-scratch |
| Integration test | vm_install_integration.py in same build job | verify_native_vm_install.py in separate vm-install job |
| Guest signal | LOCASOS_LIVE_READY / LOCASOS_INSTALLED_READY | LOCASOS_GUEST_HEALTH_OK / LOCASOS_INSTALLED_BOOT_OK |
| Diagnostics | uploads logs/screenshot on failure | exposes VM boot serial in CI |

## Recommendation
Treat #103 as the investigation baseline only: ISO build, boot smoke and separate vm-install job pass their initial stages, and the failure is traceable to service dependencies; **this is not a merge approval**. Have A fix D-Bus/Agent/Core with real guest health checks, B harden scratch-only disk identity, C own one two-stage runner, D perform security audit. Consider useful diagnostics/screenshot capture from #102 without taking its conflicting guest installer or marker contract.

## Contracts and ownership
A owns guest readiness code and ISO build adjustments, B owns guest install transaction, C owns host runner and isolated OVMF/CI, D owns threat model/static gates. E resolves shared workflow ownership and integration ordering. Recommend merging D safeguards first where independent, then A readiness, B transaction, C final end-to-end gate; after integration review and **explicit human approval** only. Keep existing #102/#103 unmerged until supersession documented.

## Evidence
PR #102 workflow run 40441635610: verified ISO/EFI metadata followed by missing markers after 120s. PR #103 workflow run 40501002087: build succeeds, vm-install fails; logs show recurring dbus.service failure, locas-agent fail, locas-core dependency failure and no LOCASOS_GUEST_HEALTH_OK. Neither PR proves disk-only reboot.
