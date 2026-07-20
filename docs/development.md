# Development

OmniSSHAgent is a Windows-only Go 1.25.6 module. `internal/backend` defines the
SSH Agent contract; `internal/interfaces` contains Pageant and Cygwin adapters;
`internal/app` owns state and lifecycle; and `internal/tray` owns its Win32
message thread.

The tray implementation follows the repository's
`examples/win-tray-sample-fyne-style`: a hidden normal top-level owner window,
legacy notification callbacks, `SetForegroundWindow` before `TrackPopupMenu`,
and menu selection through `WM_COMMAND`.

The tray and executable use the original OmniSSHAgent icon salvaged from the
archived Wails build's `build/windows/icon.ico`. The exact ICO is embedded by
`internal/tray`, while the checked-in Windows COFF resource is regenerated with:

```powershell
go generate ./cmd/omnisshagent
```

Pageant startup conflicts are represented by the Degraded tooltip/menu state
and the log. The MVP does not also show a one-time balloon notification.

Build metadata can be injected with:

```powershell
go build -ldflags="-X github.com/masahide/OmniSSHAgent/internal/cli.Version=v0.1.0 -X github.com/masahide/OmniSSHAgent/internal/cli.Commit=$env:GITHUB_SHA -X github.com/masahide/OmniSSHAgent/internal/cli.BuildTime=2026-07-20T00:00:00Z" ./cmd/omnisshagent
```

Never log protocol payloads, private keys, passphrases, or signing data.
Windows E2E steps are in [testing.md](testing.md).

## Legacy baseline

The archived implementation under `old/` was checked on 2026-07-20. Its
package tests passed except for the root package, and both `go test ./...` and
`go build ./...` stopped at `main.go:28:12` because the archived
`build/appicon.png` embed input is absent. This is the recorded migration
baseline. The old ICO was recovered from repository history for visual
continuity, but the new MVP has no Wails runtime or build dependency.
