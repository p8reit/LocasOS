# LOCAS OS WSL2 preview smoke test. Run from Windows PowerShell.
[CmdletBinding()]
param(
    [string]$DistributionName = "LocasOS",
    [ValidateRange(1,65535)][int]$Port = 59420
)
$ErrorActionPreference = "Stop"
if ($DistributionName -ne "LocasOS") { throw "Only the dedicated LocasOS distribution is supported." }
Write-Host "Checking Linux system services..."
& wsl.exe -d $DistributionName -u root -- systemctl is-active locas-firstboot locas-agent locas-core
if ($LASTEXITCODE -ne 0) { throw "A LOCAS service is not active. Run .\diagnose.ps1." }
Write-Host "Checking Linux TCP listener..."
& wsl.exe -d $DistributionName -u root -- ss -lnt
if ($LASTEXITCODE -ne 0) { throw "Could not inspect Linux TCP listeners." }
Write-Host "Testing Windows to WSL localhost on port $Port..."
$endpoint = "http://127.0.0.1:$Port/healthz"
try {
    $response = Invoke-RestMethod -Uri $endpoint -TimeoutSec 8 -NoProxy
    if ($response.component -ne "locas-core" -or $response.status -ne "ok") {
        throw "Unexpected health response from $endpoint; verify that another app is not using the port."
    }
    Write-Host "PASS: $endpoint returned LOCAS core healthy."
} catch {
    Write-Warning "Windows localhost health failed: $_"
    Write-Warning "WSL can stop when no sessions are active. Open another terminal with 'wsl -d LocasOS -u root -- bash', leave it open, then retry."
    throw
}
