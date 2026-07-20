# Migrating from Legacy OmniSSHAgent

This guide is for users moving from OmniSSHAgent 0.6.x or earlier to the
redesigned 0.7.0 and later releases.

The redesigned OmniSSHAgent is not a key store. It uses an existing
OpenSSH-compatible agent, such as the Windows OpenSSH Authentication Agent or
1Password SSH Agent, and exposes Pageant and Cygwin/MSYS2 compatibility
interfaces. Legacy settings and stored passphrases are not imported
automatically.

Read [Why OmniSSHAgent Is Being Redesigned](why-omnisshagent-is-being-redesigned.md)
before migrating.

## Use PowerShell for Windows Commands

Run every Windows command in this guide from **Windows PowerShell** or
**PowerShell 7**, not Command Prompt (`cmd.exe`) or Git Bash.

Open the Start menu, type **PowerShell**, and select **Windows PowerShell** or
**PowerShell**. The prompt normally begins with `PS`, for example:

```text
PS C:\Users\you>
```

Before continuing, run:

```powershell
(Get-Process -Id $PID).ProcessName
$PSVersionTable.PSVersion
```

The first command must return `powershell` or `pwsh`. The second command must
display a PowerShell version; Windows PowerShell 5.1 or later is recommended.
If either command is reported as unrecognized, close that window and open
PowerShell.

Commands under the WSL2 sections are the exception: run fenced `bash` commands
inside the applicable WSL2 distribution.

## Before You Begin

Do not delete the legacy installation, its settings, Credential Manager
entries, or private-key files until the new installation has passed the
verification steps in this guide.

The migration has four parts:

1. Inventory the legacy installation.
2. Move keys to an OpenSSH-compatible backend.
3. Install and configure the redesigned OmniSSHAgent.
4. Update clients and remove legacy components after verification.

## 1. Run the Read-Only Diagnostic

From a repository checkout:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\check-legacy-install.ps1
```

Or run the current diagnostic directly:

```powershell
irm https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/scripts/check-legacy-install.ps1 | iex
```

The diagnostic does not stop processes, change services, edit shell profiles,
delete files, or read stored passphrases. It reports:

- running legacy or current OmniSSHAgent processes;
- legacy `settings.json` files and referenced private-key files;
- Credential Manager target names, without reading their secret values;
- Windows OpenSSH Authentication Agent status and startup mode;
- legacy and current socket files;
- Startup-folder and `HKCU\...\Run` entries;
- installed-application records; and
- user and machine `SSH_AUTH_SOCK` values.

WSL inspection is disabled by default because querying a distribution starts
it temporarily. From a downloaded checkout, opt in with:

```powershell
.\scripts\check-legacy-install.ps1 -IncludeWSL
```

The WSL check reads common shell profiles and reports legacy proxy processes,
files, and socket paths. It does not edit the distribution.

Save the diagnostic output and make a separate backup copy of every reported
legacy `settings.json` before continuing.

## 2. Understand What Changes

| Legacy setting or feature | Redesigned equivalent |
| --- | --- |
| Built-in in-memory keyring | Windows OpenSSH, 1Password, or another OpenSSH-compatible agent |
| Saved private-key paths | Add the original key files to the selected backend |
| Saved passphrases | Not migrated; enter them again in the selected backend |
| `PageantAgent` | `interfaces.pageant.enabled` |
| `CygWinAgent` | `interfaces.cygwin.enabled` |
| `CygWinSocketPath` | `interfaces.cygwin.socket_path` |
| `DebugLog` | `logging.level = "debug"` when enabled |
| Named Pipe server | Removed; the Named Pipe is now the backend |
| `ProxyModeOfNamedPipe` | The redesigned architecture always uses an OpenSSH-compatible backend |
| WSL1 Unix socket | Not supported |
| `wsl2-ssh-agent-proxy` and `omni-socat` | Use Pipeferry for WSL2 |
| Legacy Startup-folder shortcut | Use **Start with Windows** in the tray menu |

`StartHidden`, `NamedPipeAgent`, `UnixSocketAgent`, and `UnixSocketPath` have no
direct equivalents.

`ShowBalloon` has no current equivalent and is not migrated. Signing
notifications are a future feature. The reserved
`tray.show_sign_notifications` value has no effect in the current release and
should remain disabled.

## 3. Stop the Legacy Application

Quit OmniSSHAgent from its legacy tray menu. Confirm that no legacy instance is
still running:

```powershell
Get-CimInstance Win32_Process -Filter "Name = 'OmniSSHAgent.exe'" |
  Select-Object ProcessId, ExecutablePath
```

Also stop PuTTY Pageant or another Pageant-compatible agent while testing. Only
one process can own the `Pageant` window class.

Do not start the redesigned OmniSSHAgent while the legacy process still owns
`\\.\pipe\openssh-ssh-agent`, Pageant, or its socket files.

## 4. Uninstall the Legacy Windows Application

Back up every reported `settings.json` and keep a copy of the legacy installer
before uninstalling. The legacy and redesigned installers may use the same
directory:

```text
%LOCALAPPDATA%\Programs\OmniSSHAgent
```

Uninstall the legacy application before installing the redesigned version.
Otherwise, running the legacy uninstaller later may remove files belonging to
the redesigned installation.

1. Quit the legacy tray application and confirm its process has stopped.
2. Open **Settings > Apps > Installed apps**:

   ```powershell
   Start-Process "ms-settings:appsfeatures"
   ```

3. Search for **OmniSSHAgent** and select **Uninstall**.
4. If more than one entry is present, compare its version and install location
   with the **Installed application records** section from
   `check-legacy-install.ps1`. Remove only the legacy entry.
5. Confirm that the legacy process and installed-application record are gone
   before installing the redesigned version.

The repository's current `uninstall.ps1` is only for the redesigned
PowerShell installation. It does not know how to run the legacy Wails
uninstaller and must not be used as a substitute for this step.

### If Installed Apps Has No Legacy Entry

Run `check-legacy-install.ps1` and inspect its `UninstallString` and
`InstallLocation` output. If the referenced legacy uninstaller still exists,
inspect that exact path and run the executable directly. Do not pass an
untrusted `UninstallString` to `Invoke-Expression`.

If neither an Installed Apps entry nor a legacy uninstaller exists:

1. Confirm the legacy process is stopped.
2. Preserve `settings.json`, the original private-key files, and the legacy
   installer needed for rollback.
3. Remove a Startup-folder shortcut only when its target is the confirmed
   legacy executable.
4. Delete only the confirmed legacy installation directory.
5. Leave `%APPDATA%`, Credential Manager entries, and private-key files in
   place until the redesigned installation passes verification.

Do not manually delete `%LOCALAPPDATA%\Programs\OmniSSHAgent` after the
redesigned version has been installed there.

## 5. Select and Prepare a Backend

### Option A: Windows OpenSSH Authentication Agent

Legacy installation instructions told users to stop and disable this service.
Open an elevated PowerShell window and restore it:

```powershell
Set-Service ssh-agent -StartupType Automatic
Start-Service ssh-agent
Get-Service ssh-agent
```

For every private-key path reported by the diagnostic, add the key:

```powershell
ssh-add C:\path\to\private-key
ssh-add -l
```

Enter passphrases when prompted. The migration does not extract them from
Credential Manager.

### Option B: 1Password SSH Agent

Enable the 1Password SSH Agent and confirm that it exposes:

```text
\\.\pipe\openssh-ssh-agent
```

Add or import the required keys using 1Password. Do not also start the Windows
OpenSSH Authentication Agent when both applications are configured to own the
same Named Pipe.

## 6. Install the Redesigned OmniSSHAgent

Run:

```powershell
irm https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/install.ps1 | iex
```

The installer places the application in:

```text
%LOCALAPPDATA%\Programs\OmniSSHAgent
```

The first start creates:

```text
%APPDATA%\OmniSSHAgent\config.toml
```

Check **Start with Windows** in the tray menu if required. Remove the legacy
Startup-folder shortcut so that it cannot launch the old executable at the
next sign-in.

## 7. Configure Compatibility Interfaces

The default configuration enables Pageant and Cygwin/MSYS2:

```toml
[interfaces.pageant]
enabled = true

[interfaces.cygwin]
enabled = true
socket_path = ""
```

An empty Cygwin path uses:

```text
%USERPROFILE%\.ssh\omnisshagent-cygwin.sock
```

You may temporarily copy the legacy `CygWinSocketPath` into
`interfaces.cygwin.socket_path`, but moving clients to the new default path is
recommended.

For Git Bash:

```bash
export SSH_AUTH_SOCK="$(cygpath -u "$USERPROFILE/.ssh/omnisshagent-cygwin.sock")"
ssh-add -l
```

Update persistent user or machine `SSH_AUTH_SOCK` values and shell profiles
that still reference `OmniSSHCygwin.sock` or `OmniSSHAgent.sock`.

## 8. Migrate WSL2

The redesigned OmniSSHAgent does not ship `wsl2-ssh-agent-proxy`,
`omni-socat`, WSL1 sockets, or PowerShell Named Pipe proxy commands.

Preview removal of the legacy `wsl2-ssh-agent-proxy`:

```bash
curl -fsSL https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/scripts/uninstall-legacy-wsl2.sh |
  sh -s -- --dry-run
```

Then stop its process and uninstall it:

```bash
curl -fsSL https://raw.githubusercontent.com/masahide/OmniSSHAgent/main/scripts/uninstall-legacy-wsl2.sh |
  sh
```

The uninstaller:

- identifies only a process whose executable is
  `$HOME/wsl2-ssh-agent-proxy/wsl2-ssh-agent-proxy`;
- sends `TERM`, waits up to five seconds, and sends `KILL` only if required;
- backs up `.bashrc`, `.zshrc`, `.profile`, and Fish configuration before
  removing lines containing `wsl2-ssh-agent-proxy`;
- removes `$HOME/wsl2-ssh-agent-proxy`; and
- removes the legacy socket and its directory only when the directory is
  empty.

It does not remove `omni-socat`, unrelated processes, private keys, or other
files in `.ssh`.

The legacy profile entry typically looks like:

```bash
source "$HOME/wsl2-ssh-agent-proxy/ubuntu.wsl2-ssh-agent-proxy.sh"
```

If `omni-socat` was installed separately, review these paths manually rather
than deleting them as part of the proxy uninstall:

```text
$HOME/omni-socat
$HOME/.ssh/agent.sock
```

For WSL2, follow the
[Pipeferry OpenSSH agent guide](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md).
WSL1 integration has no replacement in the redesigned OmniSSHAgent.

## 9. Verify the Migration

Perform all applicable checks before cleanup:

1. `Get-Service ssh-agent` reports `Running`, or the selected alternative
   backend is running.
2. `ssh-add -l` lists the expected keys.
3. PuTTY or WinSCP authenticates through the Pageant interface.
4. Git Bash, MSYS2, or Cygwin runs `ssh-add -l` through the new socket.
5. WSL2 authenticates through Pipeferry, if used.
6. Restart OmniSSHAgent and repeat the tests.
7. Sign out and in, then confirm **Start with Windows**, if enabled.

Logs for the redesigned application are in:

```text
%LOCALAPPDATA%\OmniSSHAgent\logs
```

## 10. Clean Up Legacy Components

Only after verification:

- delete its Startup-folder shortcut;
- remove old socket-description and owner files;
- remove old WSL proxy files and shell-profile entries;
- remove the legacy WebView cache, commonly under
  `%LOCALAPPDATA%\OmniSSHAgent.exe`; and
- remove obsolete Credential Manager entries identified by the diagnostic.

Legacy settings contain key IDs. The corresponding Windows Credential Manager
targets are named:

```text
OmniSSHAgent:<key-ID>
```

Delete those entries through Windows Credential Manager only after the keys
work in the new backend. Never delete the original private-key files as part
of automated cleanup.

Be careful with `%APPDATA%\OmniSSHAgent`: redesigned `config.toml` may share
that parent directory with legacy logs. Delete individual confirmed legacy
files rather than the entire directory.

## Rollback

Keep the legacy executable, settings backup, original service startup mode,
Startup shortcut information, and shell-profile changes until verification is
complete.

To roll back before cleanup:

1. Quit the redesigned OmniSSHAgent.
2. Disable **Start with Windows** for the redesigned application.
3. Run the redesigned `uninstall.ps1`.
4. Reinstall the retained legacy installer, if it was uninstalled.
5. Restore the legacy shortcut, settings, and shell-profile lines from the
   backup.
6. Restore the previous `ssh-agent` service startup mode only if the legacy
   configuration requires it.
7. Start the legacy application and verify its clients.

Do not run both versions simultaneously.
