[CmdletBinding()]
param(
 [ValidatePattern('^LocasOS-[A-Za-z0-9-]+$')][string]$DistributionName='LocasOS-DebianTest',
 [ValidateRange(1024,65535)][int]$Port=59421
)
$ErrorActionPreference='Stop'
if ($Port -eq 59420) { throw 'Production port 59420 is reserved' }
$names=@(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $names -notcontains $DistributionName) { throw "Unregistered test WSL distro: $DistributionName" }
$taskName="LocasOS-Test-$DistributionName-Keepalive"
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) { throw "Existing test task $taskName. No changes made." }
$source=Join-Path $PSScriptRoot 'Keep-LocasOS-Test-Running.ps1'
if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw 'Keepalive script not found' }
# Override only the test service, preserving Stable's port and service files.
$conf='[Service]'+[Environment]::NewLine+'Environment=LOCAS_LISTEN=127.0.0.1:'+$Port+[Environment]::NewLine
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($conf))
$setup='set -eu; install -d -m 0755 /etc/systemd/system/locas-core.service.d; printf %s '+$encoded+' | base64 -d > /etc/systemd/system/locas-core.service.d/90-wsl-test-port.conf; chmod 0644 /etc/systemd/system/locas-core.service.d/90-wsl-test-port.conf; systemctl daemon-reload; systemctl reset-failed locas-core.service; systemctl restart locas-core.service'
& wsl.exe -d $DistributionName -u root -- /bin/sh -c $setup
if ($LASTEXITCODE -ne 0) { throw 'Failed to set test Core port; no scheduled task created' }
$destination=Join-Path $env:LOCALAPPDATA ("LocasOS\Testing\"+$DistributionName)
New-Item -ItemType Directory -Force -Path $destination | Out-Null
$target=Join-Path $destination 'Keep-LocasOS-Test-Running.ps1'
Copy-Item -LiteralPath $source -Destination $target -Force
$exe=Join-Path $PSHOME 'powershell.exe'
if (-not (Test-Path -LiteralPath $exe)) { throw "Missing Windows PowerShell: $exe" }
$argsText='-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "'+$target+'" -DistributionName '+$DistributionName
$action=New-ScheduledTaskAction -Execute $exe -Argument $argsText
$trigger=New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
$settings=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
$principal=New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description 'Separate per-user LocasOS WSL test keepalive' | Out-Null
Start-ScheduledTask -TaskName $taskName
Write-Host "Created test task $taskName and test-only port 127.0.0.1:$Port"
Write-Host "Verify with Invoke-RestMethod http://127.0.0.1:$Port/healthz"
