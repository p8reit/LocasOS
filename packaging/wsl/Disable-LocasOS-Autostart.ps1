# Remove only the user-specific LocasOS scheduled task and its installed keepalive script.
[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"
$taskName = "LocasOS-User-Autostart"
$task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($task) {
    Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    Write-Host "Removed $taskName scheduled task."
} else { Write-Host "No LocasOS autostart task was registered." }
$target = Join-Path $env:LOCALAPPDATA "LocasOS\Windows\Keep-LocasOS-Running.ps1"
if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Force }
Write-Host "LocasOS distribution and data were not changed."
