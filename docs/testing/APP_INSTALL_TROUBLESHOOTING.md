# Ollama and Open WebUI installation troubleshooting

An `uncertain:open-webui` result means installation may have changed state before a verified outcome. It does not establish that the application is usable. Preserve models, container volumes, the WebUI database and secret key, and audit journals. Do not reset or re-import the distribution to clear this status.

## Confirm the listener first

The reported test appliance could not bind `127.0.0.1:11434`. Read-only checks identified a Windows `ollama.exe` listener on the same port. This explains the appliance service's repeated bind failure; it does not prove WebUI recovery.

Run from PowerShell:

```powershell
wsl -d LocasOS -u root -- journalctl -u locas-ollama.service -n 40 --no-pager
wsl -d LocasOS -u root -- ss -lntp 'sport = :11434'
Get-NetTCPConnection -State Listen -LocalPort 11434 | Select-Object LocalAddress,LocalPort,OwningProcess
# Substitute the PID reported above; process IDs can change.
Get-Process -Id 10904 | Select-Object Id,ProcessName,Path
```

Do not stop an unidentified listener. WSL may show a listener without its owning process. Review logs locally before sharing: host names, paths, and application output may contain private information.

## Optional separate appliance port

This procedure requires the complete LOS-130 fix bundle, including binaries, service units and installers. It is not supported by the earlier test build. The default remains port 11434; no automatic port migration occurs.

Choose a free port such as 11435. Check it in both Windows and WSL using the listener commands above with the chosen port. An empty successful result means no listener was observed at that moment; permissions or command errors do not establish availability. Keep Windows Ollama running.

Inside the dedicated LocasOS distribution, as root, inspect `/etc/locas/runtime.env` if it already exists and preserve its settings. Create or edit it to contain the following single setting, then set root ownership and mode 0600:

```text
OLLAMA_HOST=127.0.0.1:11435
```

Only IPv4 loopback ports 1024–65535 are accepted; ports 59420 and 59421 are reserved. Do not use a wildcard address. This file remains outside release payloads. Core, Agent, the app worker and Ollama read it through systemd. After installing the complete bundle and saving the file, run inside LocasOS as root:

```sh
systemctl daemon-reload
systemctl restart locas-ollama.service
systemctl restart locas-agent.service locas-core.service
systemctl is-active locas-ollama.service
curl --noproxy '*' --fail http://127.0.0.1:11435/api/version
```

These commands restart only appliance services; they do not stop Windows Ollama. Wait for stable readiness, sign into the dashboard again, review the installation status and explicitly acknowledge an uncertain attempt before retrying. The worker receives the setting on its next systemd activation. A successful API response alone is insufficient if it belongs to another service; the new worker also checks the appliance service's socket ownership.

Manual PowerShell installer wrappers invoke scripts outside systemd and do not inherit this file. Use the updated dashboard worker for configured ports. Do not assume those wrappers will honor runtime.env.

## WebUI and Docker checks

A separate source defect affected Debian 13: installing `docker.io` with `--no-install-recommends` omitted `docker-cli`. The fix explicitly installs both. This defect was reproduced in isolated tests; the supplied appliance logs established the Ollama collision rather than a specific Docker failure.

The updated scripts wait for HTTP readiness and report fixed stages such as `ollama-port-conflict`, `webui-docker-install`, `webui-pull`, `webui-readiness` and `webui-runtime-config`. Status includes a correlation reference. Ordinary logs retain fixed stage and exit codes rather than raw installer output or credentials.

An existing WebUI container with a missing, duplicate or different `OLLAMA_BASE_URL` is deliberately blocked before start. Changing runtime.env does not migrate that container. Preserve its data and request a reviewed reconfiguration; do not delete or recreate it as a generic retry. WebUI's persisted database configuration may override its launch environment and needs separate verification.

## Acceptance after retry

Verify stable appliance Ollama readiness, a successful small model pull and chat, WebUI health and login, WebUI inference against the intended appliance endpoint, and persistence after restarting the distribution. Record exact artifact and commit in the acceptance results. Source tests and CI do not establish recovery on the operator's WSL deployment.
