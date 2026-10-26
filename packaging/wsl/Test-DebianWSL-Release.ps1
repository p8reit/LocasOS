# Read-only verification of a signed Debian 13 LocasOS WSL installer release.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$ReleaseDir,
    [Parameter(Mandatory=$true)][string]$Version
)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Expected vMAJOR.MINOR.PATCH' }
$release = [IO.Path]::GetFullPath($ReleaseDir)
foreach ($name in @('locasos-wsl-amd64.tar','locasos-install.json','locasos-install.sig','locasos-verify-install.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $release $name) -PathType Leaf)) { throw "Missing $name" }
}
& (Join-Path $release 'locasos-verify-install.exe') verify-install $Version $release
if ($LASTEXITCODE -ne 0) { throw 'Publisher verification failed; no WSL changes made.' }
$tarPath = Join-Path $release 'locasos-wsl-amd64.tar'
$identity = (& tar.exe -xOf $tarPath etc/os-release 2>$null | Out-String)
if ($identity -notmatch '(?m)^ID=debian\s*$') {
    # Debian commonly represents /etc/os-release as a symlink.
    $identity = (& tar.exe -xOf $tarPath usr/lib/os-release 2>$null | Out-String)
}
if ($LASTEXITCODE -ne 0 -or $identity -notmatch '(?m)^ID=debian\s*$' -or
    $identity -notmatch '(?m)^VERSION_ID="?13"?\s*$') {
    throw 'Signed image is not Debian 13.'
}
$wslConfig = (& tar.exe -xOf $tarPath etc/wsl.conf 2>$null | Out-String)
if ($LASTEXITCODE -ne 0 -or $wslConfig -notmatch 'systemd=true') {
    throw 'Signed WSL image has no systemd startup configuration.'
}
Write-Host "PASS: $Version publisher-signed Debian 13 WSL image."
Write-Host 'READ-ONLY: no WSL distribution has been modified.'
