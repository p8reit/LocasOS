# Non-destructive upgrade of a running LocasOS WSL appliance.
[CmdletBinding()]
param([switch]$Apply)
$ErrorActionPreference = 'Stop'
$archive = Join-Path $PSScriptRoot 'locasos-upgrade-amd64.tar'
$scriptPath = Join-Path $PSScriptRoot 'upgrade-locas.sh'
if (-not (Test-Path -LiteralPath $archive -PathType Leaf)) { throw "Upgrade archive missing: $archive" }
if (-not (Test-Path -LiteralPath $scriptPath -PathType Leaf)) { throw "Upgrade script missing: $scriptPath" }
$known = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $known -notcontains 'LocasOS') { throw "Dedicated LocasOS WSL distribution not installed." }
Write-Host "Target: LocasOS. Preserved: all /var/lib application data, existing models, Docker volumes, and administrator credentials."
Write-Host "Package: $archive"
if (-not $Apply) {
    Write-Host "PREVIEW ONLY. To apply, run: .\Upgrade-LocasOS.ps1 -Apply"
    return
}
$confirmation = Read-Host 'Type UPGRADE LOCASOS to stop/restart the core services and apply this bundle'
if ($confirmation -cne 'UPGRADE LOCASOS') { throw 'Upgrade cancelled' }
$linuxPath = (& wsl.exe -d LocasOS -u root -- wslpath -a -u $archive).Trim()
if ($LASTEXITCODE -ne 0 -or -not $linuxPath.StartsWith('/')) { throw "Cannot resolve upgrade archive inside WSL" }
# Redirect LF-normalized script to stdin (avoids Windows PowerShell carriage return).
$content = [IO.File]::ReadAllText($scriptPath).Replace("`r`n", "`n").Replace("`r", "`n")
$psi = New-Object Diagnostics.ProcessStartInfo
$psi.FileName = 'wsl.exe'
# Path is derived from Windows file path and quoted for the WSL argv parser.
$psi.Arguments = '-d LocasOS -u root -- bash -s -- "' + $linuxPath.Replace('"','') + '"'
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$proc = [Diagnostics.Process]::Start($psi)
try {
    $proc.StandardInput.Write($content)
    $proc.StandardInput.Close()
    $proc.WaitForExit()
    $code = $proc.ExitCode
} finally { $proc.Dispose() }
if ($code -ne 0) { throw "Upgrade failed (exit $code). Check journalctl and backup in /var/lib/locas-upgrades." }
Write-Host 'LOCAS OS upgraded in place. No distribution reset performed.'
