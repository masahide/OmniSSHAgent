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
- `backend.type` must be `windows-openssh`.
- `backend.pipe` accepts a short name or a full `\\.\pipe\...` path.
- `backend.connect_timeout` must be a positive Go duration.
- `interfaces.*.enabled` independently enables each compatibility interface.
- The Pageant and Cygwin/MSYS2 Boolean settings can also be changed from the
  notification-area menu and take effect after restarting OmniSSHAgent.
- `tray.show_sign_notifications` is reserved for a future feature and has no
  effect in the current release.
- An empty Cygwin path resolves to
  `%USERPROFILE%\.ssh\omnisshagent-cygwin.sock`; an override must be absolute.
- `logging.level` accepts `debug`, `info`, `warn`, or `error`.

Diagnostic commands:

```powershell
OmniSSHAgent-console.exe version
OmniSSHAgent-console.exe config-path
OmniSSHAgent-console.exe check-config
OmniSSHAgent-console.exe check-config --config C:\path\to\config.toml
```
