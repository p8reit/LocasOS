# LOCAS OS test appliance: run from the extracted GitHub Actions artifact folder.
[CmdletBinding()]
param(
    [string]$PackageDirectory = $PSScriptRoot
)
$ErrorActionPreference = "Stop"
$rootfs = Join-Path $PackageDirectory "locasos-wsl-amd64.tar"
$checksums = Join-Path $PackageDirectory "locasos-wsl-amd64.tar.sha256"
$importer = Join-Path $PackageDirectory "Install-LocasOS.ps1"
foreach ($file in @($rootfs, $checksums, $importer)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Package is missing $file" }
}
$line = (Get-Content -LiteralPath $checksums -TotalCount 1)
if ($line -notmatch '^([0-9a-fA-F]{64})\s+\*?locasos-wsl-amd64\.tar\s*$') {
    throw "Invalid checksum manifest"
}
& $importer -RootfsArchive $rootfs -Sha256 $Matches[1]
if (-not $?) { throw "LOCAS installer failed" }
