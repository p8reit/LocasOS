# Host preflight (LOS-005)

Run `bash scripts/preflight.sh` on the target Ubuntu Server host. It makes no changes. Exit 0 means prerequisites were detected; exit 1 indicates missing or inaccessible tools.

The script checks only Linux, systemd, Docker CLI/Compose, NVIDIA SMI/GPU and NVIDIA Container Toolkit. The script does not validate driver/runtime compatibility, GPU access from inside Docker, or Docker daemon permissions; those require separate tests before accepting the issue.

Do not run installer, Docker restart, or package manager commands as part of preflight.
