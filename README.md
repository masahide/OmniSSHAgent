# OmniSSHAgent

OmniSSHAgent is a Windows 11 x86-64 notification-area application that makes
the Windows OpenSSH agent available to Pageant and Cygwin/MSYS2-compatible SSH
clients. It does not store private keys or passphrases.

## Prerequisite

Start Windows OpenSSH Authentication Agent and add your keys before using
OmniSSHAgent:

```powershell
Get-Service ssh-agent
ssh-add
```

OmniSSHAgent connects to `\\.\pipe\openssh-ssh-agent` for each request, so it
automatically recovers when that agent is restarted.

## Build and run

Go 1.25.6 is required.

```powershell
go build -trimpath -o OmniSSHAgent-console.exe ./cmd/omnisshagent
go build -trimpath -ldflags="-H=windowsgui" -o OmniSSHAgent.exe ./cmd/omnisshagent
.\OmniSSHAgent.exe
```

The first run creates `%APPDATA%\OmniSSHAgent\config.toml`. The tray menu shows
the current state and can open the configuration, its directory, the log
directory, or quit cleanly.

## Clients

PuTTY, WinSCP, and TortoiseGit can use the Pageant interface while
OmniSSHAgent is running.

For Git for Windows/MSYS2, set `SSH_AUTH_SOCK` to the generated descriptor:

```bash
export SSH_AUTH_SOCK="$(cygpath -u "$USERPROFILE/.ssh/omnisshagent-cygwin.sock")"
ssh-add -l
```

See [configuration](docs/configuration.md), [testing and troubleshooting](docs/testing.md),
and [development](docs/development.md).

WSL integration is provided separately by
[Pipeferry](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md).
OmniSSHAgent does not include WSL proxy commands.

## Known limitations

Configuration changes require a restart. There is no key-management GUI,
autostart registration, installer, updater, or log retention policy. Windows
10, ARM64, and non-Windows platforms are not supported by this MVP.
