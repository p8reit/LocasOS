[CmdletBinding()]
param([ValidatePattern('^LocasOS-[A-Za-z0-9-]+$')][string]$DistributionName='LocasOS-DebianTest')
$ErrorActionPreference='Stop'
$taskName="LocasOS-Test-$DistributionName-Keepalive"
$task=Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($task) {
 if ($task.State -eq 'Running') { Stop-ScheduledTask -TaskName $taskName }
 Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}
Write-Host "Test keepalive task disabled. No WSL distribution was unregistered."
