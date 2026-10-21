[CmdletBinding()]
param([string]$DistributionName = "LocasOS")
$ErrorActionPreference = "Stop"
if ($DistributionName -ne "LocasOS") { throw "Unexpected distribution" }
& wsl.exe -d $DistributionName -- systemctl --no-pager --full status locas-firstboot locas-agent locas-core
& wsl.exe -d $DistributionName -- journalctl -u locas-firstboot -u locas-agent -u locas-core --no-pager -n 120
