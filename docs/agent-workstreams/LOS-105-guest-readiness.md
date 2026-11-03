# LOS-105: Guest boot and readiness execution contract

Status: workstream opened; implementation and full QEMU acceptance pending. Owner A; draft PR required before merge.

## Known CI fault
PR #103 job `vm-install` run 40501002087 boots Debian to multi-user.target, but dbus.service fails repeatedly; locas-agent.service fails, locas-core.service dependencies fail, and `LOCASOS_GUEST_HEALTH_OK` is not emitted. Inspect journal, units, package dependencies, firstboot, and services rather than reducing health criteria.

## Interface with Agent C
Serial beacon `LOCASOS_GUEST_HEALTH_OK` is emitted only after: systemd multi-user target active; locas-agent and locas-core active; GET http://127.0.0.1:59420/healthz returns expected healthy status. No beacons inferred from QEMU timeout/process survival. Emit explicit failure diagnostics on timeout. Installed guest needs distinguishable marker for second-stage boot; agree with C before changing.

## Tests
Fail when either service is inactive, /healthz times out, response malformed, D-Bus unavailable, marker forged or serial output missing; pass only for real guest checks. Attach CI serial logs, journal excerpts and test commands/results.

Do not modify installer transaction, security docs or host QEMU code. No physical disk writes and no public release claim.
