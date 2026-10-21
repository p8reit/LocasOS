# Opt-in LOCAL ONLY runtime install, run from Windows PowerShell.
# Installs into the isolated LocasOS distro; no Docker/Desktop/Ubuntu modifications.
[CmdletBinding()]
param([switch]$AcceptDownload)
$ErrorActionPreference = 'Stop'
if (-not $AcceptDownload) {
    throw 'Downloads the Ollama Linux archive from ollama.com into LocasOS. Re-run with -AcceptDownload to proceed.'
}
$names = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $names -notcontains 'LocasOS') { throw 'Install the dedicated LocasOS WSL distribution first.' }
# Pipe the packaged installer into bash so fixes can be applied without re-importing WSL.
$linuxScript = Join-Path $PSScriptRoot 'install-ollama.sh'
if (-not (Test-Path -LiteralPath $linuxScript -PathType Leaf)) { throw "Missing install-ollama.sh. Download the updated package." }
# Windows PowerShell's native-command pipeline can append CRLF. Normalize to LF
# and write bytes directly to stdin, avoiding a stray carriage return at EOF.
$scriptText = [System.IO.File]::ReadAllText($linuxScript).Replace("`r`n", "`n").Replace("`r", "`n")
$start = New-Object System.Diagnostics.ProcessStartInfo
$start.FileName = 'wsl.exe'
$start.Arguments = '-d LocasOS -u root -- bash -s'
$start.UseShellExecute = $false
$start.RedirectStandardInput = $true
$start.RedirectStandardOutput = $false
$start.RedirectStandardError = $false
$process = [System.Diagnostics.Process]::Start($start)
try {
    $process.StandardInput.Write($scriptText)
    $process.StandardInput.Close()
    $process.WaitForExit()
    $exitCode = $process.ExitCode
} finally {
    $process.Dispose()
}
if ($exitCode -ne 0) { throw "Ollama provisioning failed (exit $exitCode). Check: wsl -d LocasOS -- journalctl -u locas-ollama -n 80 --no-pager" }
Write-Host 'Ollama installed in LocasOS only. Open the dashboard and refresh its Local AI runtime section.'
