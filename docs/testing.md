# Testing and troubleshooting

## Automated checks

```powershell
go test ./...
go vet ./...
gofmt -w cmd internal
go mod tidy
./scripts/test-installer.ps1
go build -trimpath -o OmniSSHAgent-console.exe ./cmd/omnisshagent
go build -trimpath -ldflags="-H=windowsgui" -o OmniSSHAgent.exe ./cmd/omnisshagent
```

## Windows E2E

1. Start `ssh-agent`, add a disposable test key, and run OmniSSHAgent.
2. Confirm `ssh-add -l` through PuTTY or WinSCP's Pageant-compatible mode.
3. In Git Bash/MSYS2, set `SSH_AUTH_SOCK` to
   `$USERPROFILE/.ssh/omnisshagent-cygwin.sock` through `cygpath`, then run
   `ssh-add -l` and an SSH connection.
4. Stop `ssh-agent`; confirm only the request fails. Restart it and retry
   without restarting OmniSSHAgent.
5. Start another Pageant implementation and confirm the tray says Degraded
   while the Cygwin interface remains available.
6. Introduce invalid TOML, restart, confirm Configuration error, repair it from
   the tray menu, and restart.
7. Attempt a second OmniSSHAgent process; it must exit with code 4.
8. Quit from the tray and confirm the process, TCP listener, `.sock`, and
   `.owner` file are gone.

Run an 8-hour soak with repeated list and signing requests before release.
This soak is a manual release-owner check and remains pending until its result
is recorded.

The live Cygwin path has a build-tagged test. Start OmniSSHAgent with an
isolated configuration, point the environment variable at its socket
description, and run:

```powershell
$env:OMNISSHAGENT_CYGWIN_SOCKET = 'C:\path\to\omnisshagent-cygwin.sock'
go test -tags=e2e -v ./internal/e2e
```

## Troubleshooting

Logs are in `%LOCALAPPDATA%\OmniSSHAgent\logs`. A configuration error prevents
both interfaces from starting. A Pageant class conflict affects only Pageant.
Backend connection errors usually mean the Windows OpenSSH Authentication Agent
is stopped or the configured pipe is unavailable.

## Live test record: 2026-07-20

The Windows 11 x86-64 live test used Windows OpenSSH Agent, PuTTY `plink.exe`
0.84, and an Alpine 3.22 OpenSSH server in Docker.

- The default TOML was generated in an isolated profile.
- The build-tagged Cygwin test listed the live Agent key and produced a
  signature.
- `plink -agent` authenticated to the Docker SSH server through Pageant and
  executed a remote command.
- Git for Windows 2.55.0 (`OpenSSH_10.3p1`) used the generated Cygwin socket
  description for `ssh-add -l` and authenticated to the Docker SSH server.
  This live test caught a missing `SYSTEM | READONLY` attribute on the socket
  description; the implementation and regression test now require both
  attributes.
- A separate Pageant-compatible window owner forced OmniSSHAgent into
  `Degraded` while its Cygwin socket remained available. This test caught that
  window-class registration alone cannot detect a Pageant window owned by
  another process; startup now checks the existing `Pageant` window first.
- With Windows OpenSSH Agent stopped, OmniSSHAgent stayed alive and retained
  its Cygwin socket. Git Bash returned `agent refused operation`, and `plink`
  could no longer authenticate through Pageant.
- After the service restarted, the same OmniSSHAgent process recovered without
  a restart: Git Bash listed the key again and `plink` authenticated to Docker
  through Pageant.
- A second OmniSSHAgent process exited with code 4.
- Invalid TOML started the tray without compatibility interfaces; replacing it
  with valid TOML restored normal startup.
- Tray Quit removed the process, TCP listener, socket description, owner marker,
  and mutex; a subsequent process started successfully.
- The real `%USERPROFILE%\.ssh` directory is owned by the current user and
  inherits full-control entries only for the current user, SYSTEM, and
  Administrators. The generated socket description and owner marker therefore
  stay within the expected per-user profile boundary.

Named Pipe stop/restart recovery is also covered by the automated Windows
integration test.

The release metadata build was also verified with explicit `Version`, `Commit`,
and UTC `BuildTime` ldflags; the `version` command returned all injected values
plus `windows/amd64`.

The PowerShell installer integration test used the real GUI release build and
verified first install, Start menu shortcut creation, SHA-256 rejection,
graceful replacement of a running event-aware process, forced replacement of
an event-incompatible legacy process, and uninstall. A separate live GUI test
confirmed that `uninstall.ps1` stopped the real OmniSSHAgent with exit code 0
and removed its executable, shortcut, Cygwin socket description, and owner
marker.

Together, the live checks above and the final Windows test, vet, format,
module-tidiness, console-build, and GUI-build checks satisfy AC-01 through
AC-07. The manual soak and hosted CI result remain separate release gates.
