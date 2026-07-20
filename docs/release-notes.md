# Release notes

## Windows MVP

This release is a breaking rebuild around the Windows OpenSSH agent. Wails,
WebView2, the Svelte frontend, local private-key storage, Credential Manager
integration, `wsl2-ssh-agent-proxy`, `omni-socat`, and the former public WSL
proxy packages are removed. Existing JSON settings are not migrated; review the
new TOML [configuration](configuration.md).

WSL users should migrate to
[Pipeferry](https://github.com/masahide/pipeferry/blob/main/docs/openssh-agent.md).
