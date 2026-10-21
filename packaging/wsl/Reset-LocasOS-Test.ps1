# LOCAS OS disposable WSL2 test cleanup. Never touches Ubuntu, Docker Desktop, images, volumes or containers.
# Audit by default; destructive changes require -Remove and an explicit confirmation (or -Force).
[CmdletBinding()]
param(
    [switch]$Remove,
    [switch]$Force,
    [switch]$PurgeResidual
)
$ErrorActionPreference = 'Stop'
$distribution = 'LocasOS'
$taskName = 'LocasOS-User-Autostart'
$installPath = Join-Path $env:LOCALAPPDATA 'LocasOS\wsl'
$keepalive = Join-Path $env:LOCALAPPDATA 'LocasOS\Windows\Keep-LocasOS-Running.ps1'

function Get-LocasDistributions {
    $names = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
    if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate WSL distributions. Nothing was removed.' }
    return @($names | Where-Object { $_ })
}
function Get-LocasTask {
    return Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
}
Write-Host 'LOCAS OS test cleanup / Docker readiness'
$names = @(Get-LocasDistributions)
$installed = $names -contains $distribution
$task = Get-LocasTask
Write-Host "LocasOS installed: $installed"
Write-Host "Dedicated startup task exists: $([bool]$task)"
Write-Host "Install directory exists: $(Test-Path -LiteralPath $installPath)"
Write-Host 'Other WSL distributions (will not be modified):'
@($names | Where-Object { $_ -ne $distribution }) | ForEach-Object { Write-Host "  $_" }

if ($PurgeResidual -and -not $Remove) { throw '-PurgeResidual requires -Remove.' }
if ($Remove) {
    if (-not $Force) {
        Write-Warning 'Destructive: unregistering LocasOS permanently deletes its Linux filesystem, administrator account, and all appliance data.'
        $answer = Read-Host 'Type REMOVE LOCASOS to continue'
        if ($answer -cne 'REMOVE LOCASOS') { throw 'Cleanup cancelled; no modifications were made.' }
    }
    if ($task) {
        Write-Host 'Removing only the dedicated LOCAS OS scheduled task...'
        if ($task.State -eq 'Running') { Stop-ScheduledTask -TaskName $taskName -ErrorAction Stop }
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction Stop
    }
    if (Test-Path -LiteralPath $keepalive -PathType Leaf) {
        Remove-Item -LiteralPath $keepalive -Force
    }
    if ($installed) {
        Write-Host 'Terminating and unregistering only LocasOS...'
        & wsl.exe --terminate $distribution
        if ($LASTEXITCODE -ne 0) { throw 'Unable to terminate LocasOS; nothing else will be removed.' }
        & wsl.exe --unregister $distribution
        if ($LASTEXITCODE -ne 0) { throw 'Unable to unregister LocasOS.' }
    }
    $names = @(Get-LocasDistributions)
    if ($names -contains $distribution) { throw 'LocasOS is still registered. Check WSL state.' }
    if (Test-Path -LiteralPath $installPath) {
        if ($PurgeResidual) {
            $expected = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'LocasOS\wsl')).TrimEnd('\')
            $actual = [IO.Path]::GetFullPath($installPath).TrimEnd('\')
            if ($actual -ine $expected) { throw 'Refusing unexpected cleanup directory.' }
            $item = Get-Item -LiteralPath $installPath -Force
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
                throw 'Refusing to purge a junction or symlink.'
            }
            Write-Host "Purging leftover dedicated import directory: $installPath"
            Remove-Item -LiteralPath $installPath -Recurse -Force
        } else {
            Write-Warning "Residual import directory remains: $installPath"
            Write-Warning 'Inspect it before using -Remove -PurgeResidual -Force.'
        }
    }
}

Write-Host ''
Write-Host 'Docker readiness (read only; no Docker cleanup is performed):'
$docker = Get-Command docker.exe -ErrorAction SilentlyContinue
if (-not $docker) { $docker = Get-Command docker -ErrorAction SilentlyContinue }
if ($docker) {
    & $docker.Source version --format '{{.Server.Version}}' 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host 'Docker daemon is reachable. Existing Docker resources are preserved.'
    } else {
        Write-Warning 'Docker CLI found, but daemon is not reachable; start Docker Desktop if needed.'
    }
} else {
    Write-Warning 'Docker CLI not found on PATH. This does not prevent WSL import.'
}
Write-Host ''
Write-Host 'Final WSL inventory:'
& wsl.exe --list --verbose
Write-Host ''
if ($Remove -and -not (Test-Path -LiteralPath $installPath)) {
    Write-Host 'READY: LocasOS removed and import path available. Extract the next test artifact and run Install-TestPackage.ps1.'
} elseif (-not $Remove) {
    Write-Host 'AUDIT ONLY. To remove disposable LocasOS test data: .\Reset-LocasOS-Test.ps1 -Remove'
} else {
    Write-Warning 'Review leftover LocasOS import directory before the next installation.'
}
