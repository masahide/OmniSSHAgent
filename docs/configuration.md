# Configuration

The default file is `%APPDATA%\OmniSSHAgent\config.toml`. Unknown fields are
rejected. Changes take effect after quitting and restarting OmniSSHAgent.

```toml
version = 1

[backend]
type = "windows-openssh"
pipe = "openssh-ssh-agent"
connect_timeout = "5s"

[interfaces.pageant]
enabled = true

[interfaces.cygwin]
enabled = true
socket_path = ""

[tray]
show_sign_notifications = false

[logging]
level = "info"
```

- `version` must be `1`.
- `backend.type` accepts `windows-openssh` or `embedded`.
- With `windows-openssh`, `backend.pipe` accepts a short name or a full
  `\\.\pipe\...` path, and `backend.connect_timeout` must be a positive Go
  duration.
- With `embedded`, `backend.pipe` and `backend.connect_timeout` are ignored.
  They may remain in the file so switching back preserves the external backend
  settings. OmniSSHAgent exposes the fixed `\\.\pipe\openssh-ssh-agent` pipe.
- `interfaces.*.enabled` independently enables each compatibility interface.
- The Pageant and Cygwin/MSYS2 Boolean settings can also be changed from the
  notification-area menu or the Settings dialog. Interface and backend changes
  take effect after restarting OmniSSHAgent.
- `tray.show_sign_notifications` is reserved for a future feature and has no
  effect in the current release.
- An empty Cygwin path resolves to
  `%USERPROFILE%\.ssh\omnisshagent-cygwin.sock`; an override must be absolute.
- `logging.level` accepts `debug`, `info`, `warn`, or `error`.

## Embedded backend

The minimal embedded configuration is:

```toml
[backend]
type = "embedded"
```

The embedded keyring is process-local and ephemeral:

- keys are never persisted or automatically reloaded;
- `ssh-add`, KeePassXC add/remove, Pageant, and Cygwin/MSYS2 requests use the
  same in-memory backend;
- lifetime constraints are supported;
- confirm-before-use requests fail explicitly; and
- lock and unlock use the SSH agent keyring behavior.

Only one process can own the standard OpenSSH Named Pipe. OmniSSHAgent does not
stop or disable the Windows OpenSSH Authentication Agent. If another agent owns
the pipe, the OpenSSH interface reports a startup failure and the application
state becomes **Degraded** while unrelated enabled interfaces continue.

Diagnostic commands:

```powershell
OmniSSHAgent-console.exe version
OmniSSHAgent-console.exe config-path
OmniSSHAgent-console.exe check-config
OmniSSHAgent-console.exe check-config --config C:\path\to\config.toml
```
