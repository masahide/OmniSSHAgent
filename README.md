# OmniSSHAgent

> [!IMPORTANT]
> **Upgrading from an earlier version of OmniSSHAgent?**
> The project has been redesigned around the Windows OpenSSH agent, and its
> responsibilities and configuration have changed. Before upgrading, read
> [Why OmniSSHAgent Is Being Redesigned](docs/why-omnisshagent-is-being-redesigned.md)
> and follow the [legacy migration guide](docs/migration-from-legacy.md).

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

## Install

Open PowerShell and run:

```powershell
irm https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/install.ps1 | iex
```

Administrator privileges are not required. The installer downloads the latest
Windows x86-64 release, verifies its SHA-256 checksum, installs it under
`%LOCALAPPDATA%\Programs\OmniSSHAgent`, adds a Start menu shortcut, and starts
the notification-area application.

To update, run the same command again. The installer asks a current
OmniSSHAgent process to shut down cleanly, replaces the executable, and starts
the new version.

## Build from source

Go 1.25.6 is required.

```powershell
go build -trimpath -o OmniSSHAgent-console.exe ./cmd/omnisshagent
go build -trimpath -ldflags="-H=windowsgui" -o OmniSSHAgent.exe ./cmd/omnisshagent
.\OmniSSHAgent.exe
```

The first run creates `%APPDATA%\OmniSSHAgent\config.toml`. The tray menu shows
the current state and can open the configuration, its directory, the log
directory, or quit cleanly. Check **Start with Windows** in the tray menu to
start OmniSSHAgent automatically when the current user signs in. This setting
does not require administrator privileges. The Pageant and Cygwin/MSYS2
Boolean settings can also be checked or unchecked in the tray menu; TOML
changes take effect after restarting OmniSSHAgent.

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

## Uninstall

Run this PowerShell one-liner:

```powershell
irm https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/uninstall.ps1 | iex
```

Configuration and logs are retained. Remove `%APPDATA%\OmniSSHAgent` and
`%LOCALAPPDATA%\OmniSSHAgent` separately if they are no longer needed.
The uninstaller stops an installed, running OmniSSHAgent and removes its
**Start with Windows** registration before removing it.

## Known limitations

Configuration changes require a restart. There is no key-management GUI,
automatic updater, Authenticode signature, or log retention policy. Windows
10, ARM64, and non-Windows platforms are not supported by this MVP.
