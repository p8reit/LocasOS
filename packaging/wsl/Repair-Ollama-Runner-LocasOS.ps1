# Repairs incomplete Ollama runtime without deleting existing models.
[CmdletBinding()]
param([switch]$AcceptDownload)
$ErrorActionPreference = "Stop"
if (-not $AcceptDownload) { throw "Opt-in required: -AcceptDownload (downloads official Ollama runtime)" }
$script = Join-Path $PSScriptRoot "repair-ollama-runner.sh"
if (-not (Test-Path -LiteralPath $script)) { throw "Missing repair-ollama-runner.sh" }
$body = [System.IO.File]::ReadAllText($script).Replace("`r`n","`n").Replace("`r","`n")
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "wsl.exe"
$psi.Arguments = "-d LocasOS -u root -- bash -s"
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$proc = [System.Diagnostics.Process]::Start($psi)
try {
  $proc.StandardInput.Write($body)
  $proc.StandardInput.Close()
  $proc.WaitForExit()
  $status = $proc.ExitCode
} finally { $proc.Dispose() }
if ($status -ne 0) { throw "Ollama runner repair failed (exit $status). Existing models are preserved." }
Write-Host "Runner repair complete. Test with: wsl -d LocasOS -u root -- /usr/local/bin/ollama run qwen2.5-coder:7b 'Say hello.'"
