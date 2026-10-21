# Explicit opt-in: installs a per-user Windows logon scheduled task.
[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"
$taskName = "LocasOS-User-Autostart"
$destination = Join-Path $env:LOCALAPPDATA "LocasOS\\Windows"
$source = Join-Path $PSScriptRoot "Keep-LocasOS-Running.ps1"
if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "Keepalive script missing from package." }
$known = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $known -notcontains "LocasOS") { throw "Install LocasOS first." }
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) { throw "Task already exists. Use Disable-LocasOS-Autostart.ps1 before reinstalling." }
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$target = Join-Path $destination "Keep-LocasOS-Running.ps1"
Copy-Item -LiteralPath $source -Destination $target -Force
$ps = Join-Path $PSHOME "powershell.exe"
$arguments = '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + $target + '"'
$action = New-ScheduledTaskAction -Execute $ps -Argument $arguments
$trigger = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\\$env:USERNAME"
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\\$env:USERNAME" -LogonType Interactive -RunLevel Limited
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description "Opt-in per-user keepalive for dedicated LocasOS WSL appliance" | Out-Null
Start-ScheduledTask -TaskName $taskName
Write-Host "Enabled LocasOS per-user autostart at Windows sign-in."
Write-Host "Current status: $( (Get-ScheduledTask -TaskName $taskName).State )"
Write-Host "Verify with .\\Test-LocasOS.ps1 then sign out/in to validate sign-in startup."
