# Repair installed Ollama service ownership and HOME without reinstalling LocasOS.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'repair-ollama.sh'
if (-not (Test-Path -LiteralPath $scriptPath -PathType Leaf)) { throw 'repair-ollama.sh is missing from this package.' }
$code = [IO.File]::ReadAllText($scriptPath).Replace("`r`n", "`n").Replace("`r", "`n")
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
if ($exitCode -ne 0) { throw "Ollama service repair failed (exit $exitCode)." }
Write-Host 'Ollama service repaired. Retry downloading qwen2.5:3b from the dashboard.'
