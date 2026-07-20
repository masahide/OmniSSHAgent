# Release notes

## Windows MVP

This release is a breaking rebuild around the Windows OpenSSH agent. Wails,
WebView2, the Svelte frontend, local private-key storage, Credential Manager
integration, `wsl2-ssh-agent-proxy`, `omni-socat`, and the former public WSL
proxy packages are removed. Existing JSON settings are not migrated; review the
new TOML [configuration](configuration.md).

WSL users should migrate to
[Pipeferry](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md).

The Windows x86-64 application can now be installed or updated for the current
user with the PowerShell one-liner in the README. The installer verifies the
GitHub Release executable against its published SHA-256 checksum. It does not
enable Windows logon autostart by default; users can opt in from the checked
notification-area menu. It does not provide background automatic updates.
The matching uninstall one-liner removes the executable and Start menu
shortcut, including any autostart registration, while preserving configuration
and logs. Install, update, and uninstall stop a running installed instance
before changing its files, preferring the application's graceful shutdown
event.
