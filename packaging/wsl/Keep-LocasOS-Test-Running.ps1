[CmdletBinding()]
param([ValidatePattern('^LocasOS-[A-Za-z0-9-]+$')][string]$DistributionName='LocasOS-DebianTest')
$ErrorActionPreference='Stop'
$known=@(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $known -notcontains $DistributionName) { throw "Unregistered test distro: $DistributionName" }
& wsl.exe -d $DistributionName -u root -- /usr/bin/sleep infinity
if ($LASTEXITCODE -ne 0) { throw "Test WSL keepalive ended: $LASTEXITCODE" }
