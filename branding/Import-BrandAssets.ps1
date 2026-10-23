# Import the approved full-resolution branding package into the local repository.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$ZipPath)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$zip = (Resolve-Path -LiteralPath $ZipPath).Path
$destination = Join-Path $PSScriptRoot 'assets'
if (Test-Path $destination) { throw "Destination already exists: $destination. Review existing brand assets; refusing overwrite." }
$staging = Join-Path $env:TEMP ("locasos-brand-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $staging -Force | Out-Null
try {
    Expand-Archive -LiteralPath $zip -DestinationPath $staging
    $required = @('locasos-logo-full.png','locasos-dashboard-logo.webp','locasos-icon-512.png','locasos-icon-256.png','locasos-icon-192.png','locasos-icon.ico','locasos-favicon.png','locasos-boot-1920x1080.png','locasos-boot-1280x720.png')
    foreach ($name in $required) {
        if (-not (Test-Path -LiteralPath (Join-Path $staging $name) -PathType Leaf)) { throw "Branding archive missing $name" }
    }
    New-Item -ItemType Directory -Path $destination | Out-Null
    foreach ($name in $required) { Copy-Item -LiteralPath (Join-Path $staging $name) -Destination (Join-Path $destination $name) }
    Write-Host "Approved artwork copied to $destination"
    Write-Host "Review the files, then commit on a separate branch with git add branding/assets"
    Write-Host "No push, tag, release, ISO build or destructive operation was performed."
} finally {
    Remove-Item -LiteralPath $staging -Recurse -Force -ErrorAction SilentlyContinue
}
