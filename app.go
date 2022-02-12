package main

import (
	"context"
	"fmt"

	"github.com/masahide/ssh-agent-win/pkg/wintray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App application struct
type App struct {
	ctx context.Context
	ti  *wintray.TrayIcon
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called at application startup
func (b *App) startup(ctx context.Context) {
	// Perform your setup here
	b.ctx = ctx
	b.ti = wintray.NewTrayIcon()
	b.ti.BalloonClickFunc = b.showWindow
	b.ti.TrayClickFunc = b.showWindow
	go b.ti.RunTray()
}

// domReady is called after the front-end dom has been loaded
func (b *App) domReady(ctx context.Context) {
	// Add your action here
}

// shutdown is called at application termination
func (b *App) shutdown(ctx context.Context) {
	b.ti.Dispose()
	// Perform your teardown here
}

// Greet returns a greeting for the given name
func (b *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s!", name)
}

func (b *App) showWindow() {
	runtime.WindowShow(b.ctx)
}

func (b *App) Quit() {
	b.ti.Dispose()
	runtime.Quit(b.ctx)
}
