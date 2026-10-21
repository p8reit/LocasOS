# Dedicated long-lived WSL session for the current Windows user.
# Runs inside an opt-in per-user scheduled task. No shell, privileged network binding, or host-wide WSL shutdown.
[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"
$known = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $known -notcontains "LocasOS") { throw "LocasOS is not installed." }
# The long-running sleep process pins only the LocasOS distribution; systemd owns the actual services.
& wsl.exe -d LocasOS -u root -- /usr/bin/sleep infinity
if ($LASTEXITCODE -ne 0) { throw "LocasOS keepalive exited with code $LASTEXITCODE" }
