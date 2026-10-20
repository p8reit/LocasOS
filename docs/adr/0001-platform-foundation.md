# ADR-0001: Platform foundation

Status: Proposed. Date: 2026-10-20.

Decision: Ubuntu Server 24.04 LTS headless, Docker Engine/Compose v2, NVIDIA Container Toolkit, Go locas-core and privileged locas-agent via restricted Unix-domain IPC, React frontend, SQLite WAL on local disk.

MVP: one host, one NVIDIA GPU, Ollama integration, cooperative GPU job admission. Exclusions: Kubernetes, hard VRAM isolation, multi-node, custom kernel, full ISO.

Security: no Docker socket in unprivileged core; agent only accepts typed allowlisted operations. WAN not required for normal local inference.

Review before declaring accepted: API contracts, storage policy, updates/rollback, and sandbox assumptions.
