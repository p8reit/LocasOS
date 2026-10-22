# Fresh install from a complete signed LocasOS GitHub Release asset set.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Version)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Expected version vMAJOR.MINOR.PATCH' }
$files = @('locasos-wsl-amd64.tar','locasos-wsl-amd64.tar.sha256','locasos-install.json','locasos-install.sig','locasos-verify-install.exe','Install-LocasOS.ps1','Enable-LocasOS-Autostart.ps1','Repair-LocasOS-Autostart.ps1','Keep-LocasOS-Running.ps1')
foreach($file in $files) { if(-not (Test-Path -LiteralPath (Join-Path $PSScriptRoot $file) -PathType Leaf)) { throw "Incomplete official release: missing $file" } }
& (Join-Path $PSScriptRoot 'locasos-verify-install.exe') verify-install $Version $PSScriptRoot
if ($LASTEXITCODE -ne 0) { throw 'Publisher signature or full rootfs checksum invalid; no WSL import performed' }
$manifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'locasos-install.json') -Raw | ConvertFrom-Json
Write-Host "Verified $Version from trusted publisher. Starting clean WSL install. Existing LocasOS will not be overwritten."
& (Join-Path $PSScriptRoot 'Install-LocasOS.ps1') -RootfsArchive (Join-Path $PSScriptRoot 'locasos-wsl-amd64.tar') -Sha256 $manifest.sha256
if (-not $?) { throw 'Clean installation failed' }
