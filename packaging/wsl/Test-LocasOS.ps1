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
    $raw = & curl.exe --silent --show-error --fail --noproxy "*" --max-time 8 $endpoint
    if ($LASTEXITCODE -ne 0) { throw "curl.exe exit code $LASTEXITCODE" }
    $response = $raw | ConvertFrom-Json
    if ($response.component -ne "locas-core" -or $response.status -ne "ok") {
        throw "Unexpected health response from $endpoint; verify that another app is not using the port."
    }
    Write-Host "PASS: $endpoint returned LOCAS core healthy."
} catch {
    Write-Warning "Windows localhost health failed: $_"
    Write-Warning "WSL can stop when no sessions are active. Open another terminal with 'wsl -d LocasOS -u root -- bash', leave it open, then retry."
    throw
}

Write-Host "Checking browser landing page..."
$page = & curl.exe --silent --show-error --fail --noproxy "*" --max-time 8 "http://127.0.0.1:$Port/"
if ($LASTEXITCODE -ne 0) { throw "Landing page HTTP request failed." }
$pageText = $page -join "`n"
if ($pageText -notmatch 'LOCAS OS' -or $pageText -notmatch 'Your infrastructure') {
    throw "Unexpected landing page content. Another service may own port $Port."
}
Write-Host "PASS: LOCAS OS landing page is available at http://127.0.0.1:$Port/"

Write-Host "Checking administrator API..."
$sessionJSON = & curl.exe --silent --show-error --fail --noproxy "*" --max-time 8 "http://127.0.0.1:$Port/api/session"
if ($LASTEXITCODE -ne 0) { throw "Administrator session status endpoint unavailable." }
$session = $sessionJSON | ConvertFrom-Json
if ($null -eq $session.configured -or $null -eq $session.authenticated) {
    throw "Unexpected administrator session response."
}
Write-Host "PASS: administrator status endpoint is available. Configured: $($session.configured)"
$unauthCode = & curl.exe --silent --output NUL --write-out "%{http_code}" --noproxy "*" --max-time 8 "http://127.0.0.1:$Port/api/telemetry"
if ($LASTEXITCODE -ne 0 -or $unauthCode -ne "401") {
    throw "Security failure: unauthenticated telemetry must return 401, got $unauthCode."
}
Write-Host "PASS: unauthenticated telemetry is denied (HTTP 401)."

$runtimeCode = & curl.exe --silent --output NUL --write-out "%{http_code}" --noproxy "*" --max-time 8 "http://127.0.0.1:$Port/api/runtime"
if ($LASTEXITCODE -ne 0 -or $runtimeCode -ne "401") {
    throw "Security failure: anonymous runtime inventory must return HTTP 401, got $runtimeCode."
}
Write-Host "PASS: anonymous runtime inventory is denied (HTTP 401)."

Write-Host "Checking appliance app-manager authentication..."
$appCode = & curl.exe --silent --output NUL --write-out "%{http_code}" --noproxy "*" --max-time 8 "http://127.0.0.1:$Port/api/apps"
if ($LASTEXITCODE -ne 0 -or $appCode -ne "401") { throw "Anonymous app inventory must return HTTP 401; got $appCode" }
$appPostCode = & curl.exe --silent --output NUL --write-out "%{http_code}" --noproxy "*" --max-time 8 -X POST -H "Content-Type: application/json" -H "X-Locas-Request: 1" --data '{"app":"ollama"}' "http://127.0.0.1:$Port/api/apps/install"
if ($LASTEXITCODE -ne 0 -or $appPostCode -ne "401") { throw "Anonymous app install must return HTTP 401; got $appPostCode" }
Write-Host "PASS: app inventory and installs are authenticated (HTTP 401 without login)."
