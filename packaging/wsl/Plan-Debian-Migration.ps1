# Read-only Ubuntu -> Debian migration planning. Does not copy data or modify distributions.
[CmdletBinding()]
param(
 [string]$SourceDistro='LocasOS',
 [string]$TargetDistro='LocasOS-DebianEA',
 [string]$ReportDir=(Join-Path $env:USERPROFILE 'LocasOS-Migration-Audit'),
 [string]$BackupArchive
)
$ErrorActionPreference='Stop'
if ($SourceDistro -ne 'LocasOS' -or $TargetDistro -notmatch '^LocasOS-[A-Za-z0-9-]+$') { throw 'Only the dedicated source and separate test target are supported.' }
$names=@(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $names -notcontains $SourceDistro -or $names -notcontains $TargetDistro) { throw 'Source or target WSL distro missing.' }
$sourceOS=(& wsl.exe -d $SourceDistro -u root -- cat /etc/os-release | Out-String)
$targetOS=(& wsl.exe -d $TargetDistro -u root -- cat /etc/os-release | Out-String)
if ($LASTEXITCODE -ne 0 -or $sourceOS -notmatch '(?m)^ID=ubuntu\s*$' -or $targetOS -notmatch '(?m)^ID=debian\s*$' -or $targetOS -notmatch '(?m)^VERSION_ID="?13"?\s*$') { throw 'Expected Ubuntu source and Debian 13 destination.' }
if ($BackupArchive) {
 $backup=[IO.Path]::GetFullPath($BackupArchive)
 if (-not (Test-Path -LiteralPath $backup -PathType Leaf) -or -not (Test-Path -LiteralPath "$backup.json" -PathType Leaf)) { throw 'Missing full WSL export or metadata.' }
 $meta=Get-Content -LiteralPath "$backup.json" -Raw | ConvertFrom-Json
 $item=Get-Item -LiteralPath $backup
 if ($meta.distro -ne 'LocasOS' -or $meta.archive -ne $item.Name -or $meta.bytes -ne $item.Length) { throw 'Backup identity/size mismatch.' }
 if ((Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash -ine $meta.sha256) { throw 'Full WSL backup SHA256 mismatch.' }
 Write-Host 'PASS: full original Ubuntu WSL export and checksum verified.'
} else {
 Write-Warning 'No full Ubuntu WSL export supplied; inventory only, NOT safe to migrate yet.'
}
$script=Join-Path $PSScriptRoot 'inventory-migration-state.sh'
if (-not (Test-Path -LiteralPath $script -PathType Leaf)) { throw 'Inventory script missing.' }
$dest=Join-Path ([IO.Path]::GetFullPath($ReportDir)) ('inventory-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))
if (Test-Path $dest) { throw 'Inventory report destination collision.' }
New-Item -ItemType Directory -Force -Path $dest | Out-Null
foreach ($d in @($SourceDistro,$TargetDistro)) {
 $psi=New-Object Diagnostics.ProcessStartInfo
 $psi.FileName='wsl.exe'
 $psi.Arguments='-d '+$d+' -u root -- /bin/bash -s'
 $psi.UseShellExecute=$false
 $psi.RedirectStandardInput=$true
 $psi.RedirectStandardOutput=$true
 $psi.RedirectStandardError=$true
 $p=[Diagnostics.Process]::Start($psi)
 try {
  $p.StandardInput.Write([IO.File]::ReadAllText($script))
  $p.StandardInput.Close()
  $output=$p.StandardOutput.ReadToEnd()
  $errors=$p.StandardError.ReadToEnd()
  $p.WaitForExit()
  if ($p.ExitCode -ne 0) { throw "Inventory failed ($d): $errors" }
  [IO.File]::WriteAllText((Join-Path $dest ($d+'.txt')),$output)
 } finally { $p.Dispose() }
}
@(
 'READ-ONLY inventory complete. No state has been copied.',
 "Source: $SourceDistro, target: $TargetDistro",
 'Transfer candidates (not approved until inspected): /var/lib/locas-core, /var/lib/locas, /var/lib/locas-ollama.',
 'Excluded: Ubuntu /etc, /usr, /bin, /lib and systemd files.',
 'Docker volumes, external bind mounts, tokens, IDs, ownership and permissions require independent review.',
 'Retain source WSL registration and full export for rollback.'
) | Set-Content -LiteralPath (Join-Path $dest 'PLAN.txt')
Write-Host "PASS: migration inventory at $dest"
Write-Host 'No distributions, services, applications or data were modified by the inventory tool.'
