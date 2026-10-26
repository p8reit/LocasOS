[CmdletBinding()]
param([string]$BackupDir = (Join-Path $env:USERPROFILE 'LocasOS-Backups'))
$ErrorActionPreference = 'Stop'
$names = @(& wsl.exe --list --quiet | ForEach-Object { $_.Trim([char]0).Trim() })
if ($LASTEXITCODE -ne 0 -or $names -notcontains 'LocasOS') { throw 'LocasOS not found. Nothing changed.' }
$osText = (& wsl.exe -d LocasOS -u root -- cat /etc/os-release | Out-String)
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect installed OS.' }
$osId = (($osText -split "\n" | Where-Object { $_ -match '^ID=' } | Select-Object -First 1) -replace '^ID=', '').Trim().Trim('"')
New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null
$resolved = [IO.Path]::GetFullPath($BackupDir)
$archive = Join-Path $resolved ("LocasOS-full-" + (Get-Date -Format 'yyyyMMdd-HHmmss') + ".tar")
if (Test-Path $archive) { throw 'Refusing to overwrite an existing backup.' }
Write-Host "Backing up $osId LocasOS; no unregister operation will occur."
& wsl.exe --terminate LocasOS
if ($LASTEXITCODE -ne 0) { throw 'Could not stop distro.' }
& wsl.exe --export LocasOS $archive
if ($LASTEXITCODE -ne 0) { throw 'Export failed, original distribution preserved.' }
$item = Get-Item -LiteralPath $archive
if ($item.Length -le 0) { throw 'Backup archive empty.' }
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
$meta = @{ format='locasos-wsl-export-v1'; source_os=$osId; distro='LocasOS'; archive=$item.Name; sha256=$hash; bytes=$item.Length }
$meta | ConvertTo-Json | Set-Content -LiteralPath "$archive.json" -Encoding utf8
Write-Host "PASS: $archive"
Write-Host "SHA256: $hash"
Write-Warning 'Existing LocasOS installation preserved. External mounts and other WSL distributions must be backed up separately.'
