# User-approved isolated Docker + Open WebUI deployment inside LocasOS.
[CmdletBinding()]
param([switch]$AcceptDownload)
$ErrorActionPreference = 'Stop'
if (-not $AcceptDownload) { throw 'Downloads Docker Engine packages and the Open WebUI image inside LocasOS. Retry with -AcceptDownload.' }
$script = Join-Path $PSScriptRoot 'install-open-webui.sh'
if (-not (Test-Path -LiteralPath $script -PathType Leaf)) { throw 'Missing install-open-webui.sh in package.' }
$code = [IO.File]::ReadAllText($script).Replace("`r`n", "`n").Replace("`r", "`n")
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = 'wsl.exe'
$psi.Arguments = '-d LocasOS -u root -- bash -s'
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$p = [Diagnostics.Process]::Start($psi)
try {
    $p.StandardInput.Write($code)
    $p.StandardInput.Close()
    $p.WaitForExit()
    $exitCode = $p.ExitCode
} finally { $p.Dispose() }
if ($exitCode -ne 0) { throw "LOCAS workspace installation failed (exit $exitCode). Inspect systemctl status docker and docker logs locas-open-webui." }
Write-Host 'LOCAS Workspace is starting at http://127.0.0.1:59421/'
