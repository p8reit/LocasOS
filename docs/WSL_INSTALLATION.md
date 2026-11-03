# LocasOS WSL installation track

The Windows 11/WSL2 appliance is a separate target from the planned native bootable LocasOS ISO. A WSL rootfs tar is **not** an ISO.

## Windows installation workspace

Use a Windows filesystem directory for each version. The bootstrap defaults to `%LOCALAPPDATA%\LocasOS\install\vX.Y.Z` and allows `-Workspace D:\LocasOS\install`. Release assets are not stored inside the WSL distribution. The registered WSL filesystem is separate from downloads and logs.

## Install from a published signed Early Access WSL release

Download only the initial `packaging/wsl/Install-LocasOS-WSL.ps1` bootstrap from the authenticated repository, inspect it, then execute in PowerShell with the appropriate allowed execution policy:

```powershell
.\Install-LocasOS-WSL.ps1 -Version vX.Y.Z -Channel EarlyAccess -Workspace D:\LocasOS\install
```

The entry point:
1. Queries the **explicit release tag**, not the default stable `latest` alias.
2. Refuses a release missing any WSL rootfs, signed manifest, verification executable, or installer dependency.
3. Downloads all files to an **empty version-specific Windows folder**, never mixing versions.
4. Runs the pinned-key Windows verifier against the manifest signature and rootfs checksum before invoking an installer.
5. Refuses to overwrite an existing `LocasOS` WSL distro.
6. Runs the existing signed-release installer from the verified folder.

`-DownloadOnly` stages and verifies a release without invoking WSL import. Nothing automatically changes PowerShell execution policy or unregisters WSL. Under managed AllSigned policy use properly Authenticode-signed scripts rather than bypassing policy. Publisher-signed package manifests do not themselves make PowerShell scripts Authenticode signed.

Check `wsl --list --verbose`, `wsl -d LocasOS -u root -- systemctl status locas-core`, and `Invoke-RestMethod http://127.0.0.1:59420/healthz`. After a separate `wsl --shutdown`, start LocasOS and repeat health checks. Confirm Windows sign-in autostart independently.

## Release policy

The `Signed LOCAS OS Release` workflow must publish every WSL artifact from the same build and verify the remote asset inventory after publication. Missing artifacts make an Early Access release invalid for new installs. A released upgrade archive cannot be used as the WSL rootfs image. Native bare-metal deployment is tracked separately.

See [Deployment Guide](DEPLOYMENT_GUIDE.md).
