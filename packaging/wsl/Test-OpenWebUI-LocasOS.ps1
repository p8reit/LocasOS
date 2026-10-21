[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
& wsl.exe -d LocasOS -u root -- systemctl is-active docker.service
if ($LASTEXITCODE -ne 0) { throw 'Isolated Docker service not active' }
& wsl.exe -d LocasOS -u root -- docker container inspect --format '{{.State.Status}}' locas-open-webui
if ($LASTEXITCODE -ne 0) { throw 'LOCAS workspace container missing' }
& curl.exe --fail --silent --show-error --noproxy '*' --max-time 10 http://127.0.0.1:59421/health
if ($LASTEXITCODE -ne 0) { throw 'LOCAS Workspace is not healthy yet; check docker logs locas-open-webui' }
Write-Host 'PASS: LOCAS Workspace health API reachable from Windows localhost.'
