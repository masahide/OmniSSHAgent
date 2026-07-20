//go:build windows

package main

// The ICO is the original OmniSSHAgent icon salvaged from the pre-MVP Wails
// build. Keep the generated COFF resource checked in so normal Go builds do not
// need the resource compiler.
//
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -ico ../../internal/tray/assets/tray.ico -o rsrc_windows_amd64.syso
