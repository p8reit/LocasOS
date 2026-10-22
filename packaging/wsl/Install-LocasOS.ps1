# LOCAS OS WSL2 appliance importer (development preview)
# A release rootfs and first-boot service are not yet published.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$RootfsArchive,
    [Parameter(Mandatory = $true)][string]$Sha256,
    [string]$InstallPath = "$env:LOCALAPPDATA\LocasOS\wsl",
    [string]$DistributionName = "LocasOS"
)
$ErrorActionPreference = "Stop"
if ($DistributionName -ne "LocasOS") { throw "Only the dedicated LocasOS distribution name is supported." }
$archive = (Resolve-Path -LiteralPath $RootfsArchive).Path
if (-not (Test-Path -LiteralPath $archive -PathType Leaf)) { throw "Archive not found" }
if ($Sha256 -notmatch '^[a-fA-F0-9]{64}$') { throw "Expected a SHA-256 digest (64 hexadecimal characters)" }
$actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash
if ($actual -ine $Sha256) { throw "Rootfs checksum mismatch; refusing import." }
$wsl = Get-Command wsl.exe -ErrorAction Stop
$autostart = Join-Path $PSScriptRoot "Enable-LocasOS-Autostart.ps1"
$repairAutostart = Join-Path $PSScriptRoot "Repair-LocasOS-Autostart.ps1"
$keepalive = Join-Path $PSScriptRoot "Keep-LocasOS-Running.ps1"
foreach ($required in @($autostart, $repairAutostart, $keepalive)) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Installer package missing required file: $required"
    }
}
$known = @(& $wsl.Source --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0) { throw "Could not list WSL distributions" }
if ($known -contains $DistributionName) { throw "LocasOS is already installed; refusing to overwrite it." }
if (Test-Path -LiteralPath $InstallPath) { throw "Install path exists; refusing to overwrite it." }
New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
try {
    & $wsl.Source --import $DistributionName $InstallPath $archive --version 2
    if ($LASTEXITCODE -ne 0) { throw "WSL import failed" }
} catch {
    # Do not unregister a distro automatically. A partial import requires explicit operator review.
    throw
}
# Keep a dedicated WSL session alive before the readiness probes. systemd services
# alone do not prevent WSL from suspending a distribution after commands exit.
try {
    if (Get-ScheduledTask -TaskName "LocasOS-User-Autostart" -ErrorAction SilentlyContinue) {
        & $repairAutostart
    } else {
        & $autostart
    }
    if (-not $?) { throw "Windows keepalive setup failed" }
} catch {
    Write-Warning "LocasOS was imported but Windows keepalive setup failed. Nothing was unregistered."
    throw
}
Write-Host "Imported $DistributionName. Starting first-boot provisioning..."
& $wsl.Source -d $DistributionName -u root -- systemctl start locas-firstboot.service
if ($LASTEXITCODE -ne 0) {
    Write-Warning "First boot failed. The imported distro was preserved for diagnostics."
    throw "Run packaging/wsl/diagnose.ps1 to inspect logs. No automatic deletion was performed."
}
# systemd service startup is asynchronous in WSL; tolerate scheduling delays.
$healthy = $false
for ($attempt = 1; $attempt -le 30; $attempt++) {
    & $wsl.Source -d $DistributionName -u root -- systemctl is-active --quiet locas-agent.service locas-core.service 2>$null
    if ($LASTEXITCODE -eq 0) {
        $healthy = $true
        break
    }
    Start-Sleep -Seconds 2
}
if (-not $healthy) {
    Write-Warning "LOCAS services did not become healthy. Import preserved for diagnostics."
    & $wsl.Source -d $DistributionName -u root -- systemctl --no-pager status locas-agent.service locas-core.service
    throw "LOCAS services not healthy after readiness checks; imported distro preserved."
}
# Verify availability from the Windows browser's side, not only service state.
# Also validate the identity of the response so another app cannot pass this probe.
$endpoint = "http://127.0.0.1:59420/healthz"
$ready = $false
for ($attempt = 1; $attempt -le 30; $attempt++) {
    try {
        $raw = & curl.exe --silent --show-error --fail --noproxy "*" --max-time 4 $endpoint 2>$null
        if ($LASTEXITCODE -eq 0) {
            $health = ($raw -join "`n") | ConvertFrom-Json
            if ($health.component -eq "locas-core" -and $health.status -eq "ok") {
                $ready = $true
                break
            }
        }
    } catch { }
    Start-Sleep -Seconds 2
}
if (-not $ready) {
    Write-Warning "Windows cannot reach a verified LOCAS dashboard on $endpoint."
    & $wsl.Source -d $DistributionName -u root -- systemctl --no-pager status locas-agent.service locas-core.service
    throw "Installation incomplete: dashboard readiness verification failed. Distro preserved; run diagnose.ps1."
}
Write-Host "PASS: LOCAS services and Windows dashboard health verified."
Write-Host "LOCAS OS installed. Open http://127.0.0.1:59420/ for first-run administrator enrollment."
