# Side-by-side Early Access test installer. Never touches or unregisters Stable LocasOS.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$ReleaseDir,
    [Parameter(Mandatory=$true)][string]$Version,
    [ValidatePattern('^LocasOS-[A-Za-z0-9-]+$')][string]$DistributionName='LocasOS-DebianEA',
    [ValidateRange(1024,65535)][int]$Port=59421,
    [string]$InstallPath
)
$ErrorActionPreference='Stop'
if ($Port -eq 59420) { throw 'Production port 59420 is reserved.' }
if (-not $InstallPath) { $InstallPath=Join-Path $env:LOCALAPPDATA ('LocasOS\Testing\wsl-'+$DistributionName) }
$dir=[IO.Path]::GetFullPath($ReleaseDir)
$testTool=Join-Path $PSScriptRoot 'Test-DebianWSL-Release.ps1'
$enableTool=Join-Path $PSScriptRoot 'Enable-LocasOS-Test-Keepalive.ps1'
if (-not (Test-Path $testTool) -or -not (Test-Path $enableTool)) { throw 'Missing verification or test keepalive utility.' }
# Verifies signed full rootfs, Debian 13 identity, and systemd before any WSL mutation.
& $testTool -ReleaseDir $dir -Version $Version
if (-not $?) { throw 'Debian signed release preflight failed.' }
$names=@(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0) { throw 'Unable to list WSL distributions.' }
if ($names -contains $DistributionName) { throw "Distribution already exists: $DistributionName. Nothing changed." }
if (Test-Path -LiteralPath $InstallPath) { throw "Install destination already exists: $InstallPath. Nothing changed." }
$taskName="LocasOS-Test-$DistributionName-Keepalive"
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) { throw "Test task already exists: $taskName" }
$rootfs=Join-Path $dir 'locasos-wsl-amd64.tar'
New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
& wsl.exe --import $DistributionName $InstallPath $rootfs --version 2
if ($LASTEXITCODE -ne 0) { throw 'Import failed. No automatic deletion or unregister; inspect the target.' }
Write-Host "Imported $DistributionName from publisher-verified Debian 13 rootfs."
try {
    # The tested keepalive also sets a test-only systemd drop-in for the listener.
    & $enableTool -DistributionName $DistributionName -Port $Port
    if (-not $?) { throw 'Test keepalive setup failed.' }
    & wsl.exe -d $DistributionName -u root -- systemctl start locas-firstboot.service
    if ($LASTEXITCODE -ne 0) { throw 'Firstboot failed.' }
    foreach ($service in @('locas-agent.service','locas-core.service')) {
        & wsl.exe -d $DistributionName -u root -- systemctl start $service
        if ($LASTEXITCODE -ne 0) { throw "Service startup failed: $service" }
    }
    $ready=$false
    for ($attempt=0; $attempt -lt 30; $attempt++) {
        $guest=(& wsl.exe -d $DistributionName -u root -- curl --fail --silent --max-time 3 "http://127.0.0.1:$Port/healthz" 2>$null | Out-String)
        if ($LASTEXITCODE -eq 0) {
            try {
                $health=$guest | ConvertFrom-Json
                if ($health.component -eq 'locas-core' -and $health.status -eq 'ok') {
                    $ready=$true
                    break
                }
            } catch {}
        }
        Start-Sleep -Seconds 2
    }
    if (-not $ready) { throw 'Debian guest health verification failed.' }
    if ((Get-ScheduledTask -TaskName $taskName).State -ne 'Running') { throw 'Windows test keepalive task not running.' }
    Write-Host "PASS: $DistributionName Core/Agent and in-guest endpoint healthy at 127.0.0.1:$Port"
    Write-Host "Check Windows: Invoke-RestMethod http://127.0.0.1:$Port/healthz"
} catch {
    Write-Warning "Installation incomplete: $_"
    Write-Warning 'New test distribution preserved for diagnostics. Original LocasOS was not modified.'
    throw
}
