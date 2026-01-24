# Repository Guidelines

## Project Structure & Module Organization
- `main.go` + `app.go` boot Wails; keep shared logic under `pkg/` so the UI layer can stay lean.
- `pkg/` houses platform helpers (`namedpipe`, `pageant`, `sshkey`, `store`, `wintray`, `unix`, etc.) and is the right place for reusable agent code.
- `cmd/` contains helper binaries (`agent-bench`, `pageant-add`, `omni-socat`, `wsl2-ssh-agent-proxy`); keep each CLI focused and small.
- `frontend/` hosts the Svelte UI (`src/`), generated bindings (`wailsjs/`), and Rollup config; treat `frontend/dist/` as derived output.
- `build/` holds Wails-specific assets (icons, manifests) described in `build/README.md`; keep overrides minimal and aligned with release needs.
- `doc/`, `hack/`, and `test/` store diagrams, WSL helper scripts, and supporting binaries/keys used by unit tests.

## Build, Test, and Development Commands
- `cd frontend && npm install` to fetch UI dependencies before running any build or dev task.
- `wails dev` runs the Go backend alongside the Rollup watcher so you can test the desktop UI locally with hot reload.
- `wails build` compiles the Go backend, bundles `frontend/dist`, and writes the production binaries into `build/bin`.
- Use `npm run build` inside `frontend/` when you need to regenerate static assets without running `wails dev`.
- When running from WSL, execute `go.exe build`/`go.exe test` instead of `go build`/`go test` so the Windows toolchain is used.

## Coding Style & Naming Conventions
- Go files follow `gofmt` formatting; run it on touched files and prefer tabs for indentation.
- Package names stay short and lowercase (e.g., `sshutil`, `muxconn`); exported identifiers get PascalCase while helpers stay unexported.
- The front-end relies on Prettier with `prettier-plugin-svelte`; run `npm run format` before committing UI work.

## Testing Guidelines
- Backend tests live alongside packages (see `pkg/sshutil/sshutil_test.go`); run `go test ./...` to cover them.
- Tests follow table-driven styles—describe each case and use `t.Run` so failures are scoped per key type or agent state.

## Commit & Pull Request Guidelines
- Follow the existing imperative commit style (e.g., `Improve error handling for Pageant initialization`) and reference issues/PRs when applicable.
- PRs should explain the change, document commands you ran (`go test ./...`, `npm run build`, etc.), and attach UI screenshots when interfaces change.

## Security & Configuration Tips
- Keep socket paths and proxies configurable; update `hack/ubuntu.wsl2-ssh-agent-proxy.*` if you need new helpers for WSL.
- Passphrases live in the Windows Credential Manager, so add no secrets or credentials to the repo when working on `store` or UI settings.
