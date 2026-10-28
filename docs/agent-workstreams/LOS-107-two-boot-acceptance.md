# LOS-107: Two-boot QEMU acceptance contract

Status: harness interface prepared; end-to-end runtime proof pending. Owner C; draft PR required.

## Stage 1
Build or fetch exact verified ISO, check digest/signature as appropriate, create isolated scratch qcow2 and writable copy of matching OVMF_VARS, use read-only OVMF_CODE and ISO, no network or host disk passthrough. Require guest health after real Core and Agent checks (Agent A contract), then invoke uniquely opt-in QEMU-only installer (Agent B contract) and require exact install completion marker.

## Stage 2
Terminate stage 1 and start a new VM using disk **without ISO or install fw_cfg marker**. Require EFI disk boot, separate installed readiness marker and Core/Agent health. Verify persistent data by creating a sentinel on data volume before reboot and reading the same value after boot. Emit stage-specific serial logs and failure artifacts.

## False positives
Reject markers printed by QEMU/host, stale logs, missing serial output, early QEMU exit, timeout, absent EFI loader, absent data partition, wrong virtual serial, network-passthrough flags and any host device references. Keep all assets temporary with cleanup.

## Current CI
PR #102: ISO booted via OVMF but live readiness and install completion absent after 120 seconds. PR #103: ISO build/smoke passes but vm-install fails with DBus/Core/Agent readiness. Do not mark acceptance until both stages pass.
