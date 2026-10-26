# Parallel Debian WSL lifecycle test

The existing Ubuntu Stable Windows installer creates `LocasOS-User-Autostart`, which runs `sleep infinity` inside the dedicated `LocasOS` WSL distribution. A manual `wsl --import LocasOS-DebianTest` does not install that task; systemd services are stopped when WSL terminates the idle distribution.

For a separate Debian test install, run the following PowerShell commands from `packaging/wsl`:

```powershell
.\Enable-LocasOS-Test-Keepalive.ps1 -DistributionName LocasOS-DebianTest -Port 59421
Get-ScheduledTask -TaskName LocasOS-Test-LocasOS-DebianTest-Keepalive
wsl -d LocasOS-DebianTest -u root -- systemctl is-active locas-core.service locas-agent.service
Invoke-RestMethod http://127.0.0.1:59421/healthz
```

This is a **test-only** scheduled task, separate from `LocasOS-User-Autostart`. The core listener is changed through a systemd drop-in inside `LocasOS-DebianTest` only. Stable remains bound to localhost port 59420.

After closing both consoles, verify port 59421 again. Then sign out and back in to validate automatic startup. If WSL localhost forwarding fails, check the guest first:

```powershell
wsl -d LocasOS-DebianTest -u root -- curl -fsS http://127.0.0.1:59421/healthz
```

Disable only the test keepalive with:

```powershell
.\Disable-LocasOS-Test-Keepalive.ps1 -DistributionName LocasOS-DebianTest
```

This does **not** unregister or modify the Stable distribution and does not remove the systemd port override from the test distribution. Linux CI validates PowerShell syntax, not actual Windows task and WSL lifetime; validate both on a disposable Windows installation before promoting this to a standard product experience.
