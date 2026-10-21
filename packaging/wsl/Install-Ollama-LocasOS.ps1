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
Get-Content -LiteralPath $linuxScript -Raw | & wsl.exe -d LocasOS -u root -- bash -s
if ($LASTEXITCODE -ne 0) { throw "Ollama provisioning failed (exit $LASTEXITCODE). Check: wsl -d LocasOS -- journalctl -u locas-ollama -n 80 --no-pager" }
Write-Host 'Ollama installed in LocasOS only. Open the dashboard and refresh its Local AI runtime section.'
