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
& wsl.exe -d LocasOS -u root -- /usr/local/sbin/locas-install-ollama
if ($LASTEXITCODE -ne 0) { throw "Ollama provisioning failed (exit $LASTEXITCODE). Check: wsl -d LocasOS -- journalctl -u locas-ollama -n 80 --no-pager" }
Write-Host 'Ollama installed in LocasOS only. Open the dashboard and refresh its Local AI runtime section.'
