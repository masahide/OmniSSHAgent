# OmniSSHAgent development guide

OmniSSHAgent is a Windows-only Go application. The MVP must not depend on Wails,
WebView2, Node.js, Svelte, local private-key storage, or WSL proxy code.

Run `go test ./...`, `go vet ./...`, and both console and `windowsgui` builds
before marking implementation tasks complete. Windows-specific code must use a
`//go:build windows` constraint. Never log SSH agent payloads, private keys,
passphrases, or signing data.

Compatibility interfaces depend only on `internal/backend.Backend`. Pageant and
the tray each own a dedicated locked OS thread. Cygwin listeners bind only to
loopback and may remove an existing socket description only after verifying its
OmniSSHAgent owner marker.
