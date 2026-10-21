# Check the isolated Ollama service, NVIDIA passthrough and model inference status.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
& wsl.exe -d LocasOS -u root -- systemctl is-active locas-ollama.service
if ($LASTEXITCODE -ne 0) { throw 'Ollama service is not active inside LocasOS.' }
& wsl.exe -d LocasOS -u root -- /usr/local/bin/ollama list
if ($LASTEXITCODE -ne 0) { throw 'Ollama local model list failed.' }
& wsl.exe -d LocasOS -u root -- /usr/local/bin/ollama ps
if ($LASTEXITCODE -ne 0) { throw 'Ollama processor inventory failed.' }
Write-Host 'Processor shows GPU after a model is loaded if offloading is supported.'
Write-Host 'Confirm GPU offload while the model is actively running. A working nvidia-smi alone is not sufficient.'
