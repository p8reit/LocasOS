# Repair only the per-user LOCAS OS scheduled keepalive task.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$taskName = 'LocasOS-User-Autostart'
$source = Join-Path $PSScriptRoot 'Keep-LocasOS-Running.ps1'
if (-not (Test-Path -LiteralPath $source)) { throw 'Missing Keep-LocasOS-Running.ps1 in package.' }
$distros = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $distros -notcontains 'LocasOS') { throw 'LocasOS WSL distro not found.' }
$targetDir = Join-Path $env:LOCALAPPDATA 'LocasOS\Windows'
New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
$target = Join-Path $targetDir 'Keep-LocasOS-Running.ps1'
Copy-Item -LiteralPath $source -Destination $target -Force
$existing = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($existing) {
    try { Stop-ScheduledTask -TaskName $taskName -ErrorAction Stop } catch { Write-Warning "Could not stop stale task: $_" }
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction Stop
}
$exe = Join-Path $PSHOME 'powershell.exe'
if (-not (Test-Path -LiteralPath $exe)) { throw "Windows PowerShell executable not found: $exe" }
$action = New-ScheduledTaskAction -Execute $exe -Argument ('-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $target + '"')
$trigger = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description 'LOCAS OS WSL keepalive; per-user, no other distributions touched' | Out-Null
Start-ScheduledTask -TaskName $taskName
$found = $false
for ($i=0; $i -lt 15; $i++) {
    # WSL can emit a non-fatal systemd root-session warning on stderr.
    # PowerShell 5.1 may promote native stderr into a terminating error.
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = 'wsl.exe'
    $psi.Arguments = '-d LocasOS -u root -- pgrep -f "^/usr/bin/sleep infinity$"'
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $probe = [System.Diagnostics.Process]::Start($psi)
    try {
        $output = $probe.StandardOutput.ReadToEnd()
        $warningText = $probe.StandardError.ReadToEnd()
        $probe.WaitForExit()
        if ($probe.ExitCode -eq 0 -and -not [string]::IsNullOrWhiteSpace($output)) {
            $found = $true
            break
        }
    } finally {
        $probe.Dispose()
    }
    Start-Sleep -Seconds 1
}
if (-not $found) {
    Write-Warning 'Task recreated but keepalive process not detected. Inspect Get-ScheduledTaskInfo and task action.'
    throw 'LOCAS keepalive not verified.'
}
Write-Host 'PASS: Windows task recreated and WSL keepalive sleep process verified.'
Write-Host 'Close the installer terminal and test http://127.0.0.1:59420/healthz again.'
