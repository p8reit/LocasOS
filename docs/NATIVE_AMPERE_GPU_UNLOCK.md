# Native Ampere GPU Unlock Integration

Status: **design / not enabled in released images**.

Upstream: https://github.com/Zagitta/gpu-unlock (review and pin a vetted commit before packaging).

## Goal

Ship optional, first-party-managed support for Zagitta's `gpu_unlock` early-boot Linux kernel module with LocasOS native Debian images, so supported NVIDIA Ampere consumer cards can use the higher FP16/BF16/TF32 matrix issue rates. This is a hardware optimization, not a guaranteed LLM token/s improvement.

## Platform scope

- **Native bare metal**: the only initial execution target. Package integration may be included in the native image, but activation requires verified hardware, boot-chain readiness, and an explicit acceptance gate during early-access development.
- **Native VM**: detection/reporting only by default. PCI passthrough must be positively verified before eligibility; testing in a VM does not qualify the feature for general activation.
- **Windows/WSL2**: feature marked **unsupported**. A module in the WSL guest cannot safely be relied on to take over the Windows-owned GPU before the host's NVIDIA driver. Never change WSL boot parameters, load the module, or claim an unlock occurred.
- Non-NVIDIA, unsupported NVIDIA, and unsupported PCI IDs: no-op.

## Proposed LocasOS defaults

- Image capability: `gpu_unlock.available=true` only when the vetted module, matching kernel headers, firmware payload requirements, and initramfs integration have been validated.
- Installer default for *compatible native bare metal*: offer `auto` acceleration profile, but **do not activate silently before release testing and informed confirmation**; support `disabled`, `enabled` and `auto`.
- Source acquisition: pinned immutable upstream revision and SHA-256 verified inputs during a controlled build, with license and firmware provenance review. No unpinned `curl | sh` or live GitHub clone during first boot.
- Per-GPU PCI vendor/device detection must match vetted IDs; prefer an explicit `gpu_unlock.targets=<PCI BDF>` allowlist over `all`. GA103 is untested upstream; only tested GA106 variant is RTX 3060 12GB `10de:2504`; treat other IDs as experimental until measured. Module support listings are broader than the tested matrix.
- Boot integration: `gpu_unlock` must run in the initramfs **before** `nvidia` binds, with the appropriate `gpu_unlock.targets=` kernel parameter. Loading it after NVIDIA has bound does not unlock the GPU.
- Persist per-card status in read-only dashboard diagnostics: unavailable / ineligible / eligible / installed / enabled / verified / failed / recovery-disabled. Never equate installed or module-loaded with successful unlock.
- Offer a one-boot recovery path that drops the `gpu_unlock.targets=` parameter; retain an older known-good boot entry and document power-off recovery.

## Validation and release gates

1. Static checks: kernel module packaging, pinned sources/checksums, provenance/licenses, platform detection, PCI allowlist, idempotent enable/disable, initramfs order, boot parameter persistence.
2. Native QEMU boot: unsupported GPU is a no-op; without passthrough there is no unlock; kernel/driver updates and uninstall preserve bootability.
3. Hardware QA: RTX 3090 / 3070 / RTX 3060 12GB `10de:2504` at minimum, with before/after FP16, BF16, TF32 GEMM and application-level LLM prefill/decode tests; measure power, error logs, correctness, and stability.
4. Failure injections: incompatible kernel, missing headers, Secure Boot/module signing, NVIDIA already bound, PCI reset failures, suspend/resume, boot-loop recovery, GPU not available after reboot.
5. Signed release build / rollback: module+initramfs produced and verified by build pipeline, signed with the product artifacts, with a safe upgrade path when kernel/NVIDIA versions change. Keep activation disabled if validation is incomplete.
6. Clear unsupported messaging for WSL and machines without compatible native PCI devices.

## Known risks and limits

The upstream module manipulates GPU initialization and performs a PCIe function-level reset (FLR); a display may blank temporarily and an incompatible device can fail to reinitialize. Firmware generation uses NVIDIA-derived inputs under separate terms. Review distribution/legal implications, Secure Boot signing, early initramfs boot order, resume handling, and the behavior of the native recovery environment before enabling by default.

Do not promise the upstream's maximum ~80% BF16 GEMM improvement as a universal performance gain. Inference decode is often memory-bandwidth limited. Benchmark per model/workload and record versions.

## Implementation checklist

- [ ] Native build dependency and pinned source review
- [ ] Pre-driver initramfs packaging for Debian 13 native image
- [ ] Native-only PCI detection and supported-ID policy
- [ ] LocasOS GPU acceleration config/API and dashboard status
- [ ] Activation, disable, rollback and safe boot paths
- [ ] Hardware and regression test evidence
- [ ] Signed package/update integration
- [ ] Documentation / Early Access release gate

Related: native NVIDIA/CUDA milestone #83; Debian native image design and native installer architecture.
