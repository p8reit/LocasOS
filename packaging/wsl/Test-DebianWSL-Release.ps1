[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$ReleaseDir,[Parameter(Mandatory=$true)][string]$Version)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Expected release version vMAJOR.MINOR.PATCH.' }
$release = [IO.Path]::GetFullPath($ReleaseDir)
foreach ($name in @('locasos-wsl-amd64.tar','locasos-install.json','locasos-install.sig','locasos-verify-install.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $release $name) -PathType Leaf)) { throw "Missing $name" }
}
& (Join-Path $release 'locasos-verify-install.exe') verify-install $Version $release
if ($LASTEXITCODE -ne 0) { throw 'Publisher verification failed; no WSL changes made.' }
$tar = Join-Path $release 'locasos-wsl-amd64.tar'
$identity = (& tar.exe -xOf $tar etc/os-release 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $identity -notmatch '(?m)^ID=debian\s*
    throw 'Signed release is not a Debian 13 rootfs.'
}
$wsl = (& tar.exe -xOf $tar etc/wsl.conf 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $wsl -notmatch 'systemd=true') { throw 'WSL systemd startup configuration missing.' }
Write-Host "PASS: $Version signed Debian 13 WSL rootfs and systemd configuration."
Write-Host 'READ-ONLY: no existing distribution was changed.'
) {
    # Debian images may expose /etc/os-release as a symlink to /usr/lib/os-release.
    $identity = (& tar.exe -xOf $tar usr/lib/os-release 2>&1 | Out-String)
}
if ($LASTEXITCODE -ne 0 -or $identity -notmatch '(?m)^ID=debian\s*
    throw 'Signed release is not a Debian 13 rootfs.'
}
$wsl = (& tar.exe -xOf $tar etc/wsl.conf 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $wsl -notmatch 'systemd=true') { throw 'WSL systemd startup configuration missing.' }
Write-Host "PASS: $Version signed Debian 13 WSL rootfs and systemd configuration."
Write-Host 'READ-ONLY: no existing distribution was changed.'
 -or $identity -notmatch '(?m)^VERSION_ID="?13"?\s*
    throw 'Signed release is not a Debian 13 rootfs.'
}
$wsl = (& tar.exe -xOf $tar etc/wsl.conf 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $wsl -notmatch 'systemd=true') { throw 'WSL systemd startup configuration missing.' }
Write-Host "PASS: $Version signed Debian 13 WSL rootfs and systemd configuration."
Write-Host 'READ-ONLY: no existing distribution was changed.'
) {
    throw 'Signed release is not a Debian 13 rootfs.'
}
$wsl = (& tar.exe -xOf $tar etc/wsl.conf 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $wsl -notmatch 'systemd=true') { throw 'WSL systemd startup configuration missing.' }
Write-Host "PASS: $Version signed Debian 13 WSL rootfs and systemd configuration."
Write-Host 'READ-ONLY: no existing distribution was changed.'
