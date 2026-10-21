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
Write-Host "Imported $DistributionName successfully."
Write-Host "First-boot setup and local dashboard launch are not yet available in this development preview."
