# LocasOS WSL release bootstrap. Runs on Windows, outside the WSL guest.
[CmdletBinding()]
param(
    [string]$Version,
    [ValidateSet('EarlyAccess','Stable')][string]$Channel = 'EarlyAccess',
    [string]$Workspace = (Join-Path $env:LOCALAPPDATA 'LocasOS\install'),
    [switch]$DownloadOnly
)
$ErrorActionPreference = 'Stop'
$repo = 'p8reit/LocasOS'
$required = @(
    'locasos-wsl-amd64.tar',
    'locasos-wsl-amd64.tar.sha256',
    'locasos-install.json',
    'locasos-install.sig',
    'locasos-verify-install.exe',
    'Install-Release-LocasOS.ps1',
    'Install-LocasOS.ps1',
    'Enable-LocasOS-Autostart.ps1',
    'Repair-LocasOS-Autostart.ps1',
    'Keep-LocasOS-Running.ps1',
    'Test-LocasOS.ps1'
)

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw 'GitHub CLI (gh) is required. Install it with winget install --id GitHub.cli -e, then authenticate with gh auth login.'
}
if (-not $Version) {
    $releases = @(& gh release list -R $repo --limit 50 --json tagName,isPrerelease,isDraft --jq '.[] | select(.isDraft == false) | [.tagName, .isPrerelease] | @tsv')
    if ($LASTEXITCODE -ne 0) { throw 'Unable to query GitHub releases. Run gh auth login.' }
    $choice = $releases | Where-Object {
        $fields = $_ -split "`t"
        $fields.Count -ge 2 -and ($fields[1] -eq $(if ($Channel -eq 'EarlyAccess') { 'true' } else { 'false' }))
    } | Select-Object -First 1
    if (-not $choice) { throw "No published $Channel release found." }
    $Version = ($choice -split "`t")[0]
}
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Expected version vMAJOR.MINOR.PATCH.' }
$releaseText = (& gh release view $Version -R $repo --json tagName,isPrerelease,isDraft,assets 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0) { throw "Could not inspect release $Version : $releaseText" }
$release = $releaseText | ConvertFrom-Json
if ($release.isDraft) { throw "Release $Version is a draft." }
if ($Channel -eq 'Stable' -and $release.isPrerelease) { throw "Release $Version is a prerelease, not Stable." }
$published = @($release.assets | ForEach-Object { $_.name })
$missing = @($required | Where-Object { $_ -notin $published })
if ($missing.Count -gt 0) { throw "Incomplete GitHub WSL release $Version. Missing: $($missing -join ', '). No files downloaded and no WSL changes made." }

$directory = Join-Path $Workspace $Version
if (Test-Path -LiteralPath $directory) {
    if (@(Get-ChildItem -LiteralPath $directory -Force -ErrorAction Stop).Count -gt 0) {
        throw "Installation staging folder already contains files: $directory. Choose an empty Workspace; release assets must not be mixed."
    }
} else {
    New-Item -Path $directory -ItemType Directory -Force | Out-Null
}

& gh release download $Version -R $repo --dir $directory
if ($LASTEXITCODE -ne 0) { throw "GitHub release download failed; delete or quarantine the incomplete staging directory: $directory" }
$localMissing = @($required | Where-Object { -not (Test-Path -LiteralPath (Join-Path $directory $_) -PathType Leaf) })
if ($localMissing.Count -gt 0) { throw "Incomplete download: $($localMissing -join ', '). No WSL changes made." }

# Verify the publisher's detached signature and the whole rootfs archive before invoking ANY installer script.
$verifier = Join-Path $directory 'locasos-verify-install.exe'
& $verifier verify-install $Version $directory
if ($LASTEXITCODE -ne 0) { throw 'Release verification failed. No WSL changes made.' }
Write-Host "Signed LocasOS $Version WSL package verified in $directory"
if ($DownloadOnly) { return }
if (@(& wsl.exe --list --quiet) -match '^LocasOS\s*$') {
    throw 'WSL distribution LocasOS already exists. A fresh install will not overwrite it.'
}
$installer = Join-Path $directory 'Install-Release-LocasOS.ps1'
# The bootstrap is the entry point. Explicit Process-scoped RemoteSigned changes are not made automatically.
& $installer -Version $Version
if (-not $?) { throw "Installer failed; see $directory and WSL logs." }
