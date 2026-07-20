//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/masahide/OmniSSHAgent/internal/app"
	"github.com/masahide/OmniSSHAgent/internal/backend/openssh"
	"github.com/masahide/OmniSSHAgent/internal/cli"
	"github.com/masahide/OmniSSHAgent/internal/config"
	"github.com/masahide/OmniSSHAgent/internal/interfaces"
	"github.com/masahide/OmniSSHAgent/internal/interfaces/cygwin"
	"github.com/masahide/OmniSSHAgent/internal/interfaces/pageant"
	"github.com/masahide/OmniSSHAgent/internal/logging"
	"github.com/masahide/OmniSSHAgent/internal/singleton"
	"github.com/masahide/OmniSSHAgent/internal/tray"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	result := cli.Run(args, os.Stdout, os.Stderr)
	if !result.StartApplication {
		return result.Code
	}
	mutex, err := singleton.Acquire(`Local\OmniSSHAgent`)
	if errors.Is(err, singleton.ErrAlreadyRunning) {
		return cli.ExitAlreadyRunning
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cli.ExitWindows
	}
	defer mutex.Close()

	configPath, err := config.DefaultConfigPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cli.ExitInternal
	}
	logDir, err := config.DefaultLogDirectory()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cli.ExitInternal
	}
	logger, logCloser := logging.New(logDir, slog.LevelInfo)
	defer func() { _ = logCloser.Close() }()
	logger.Info("application starting", "version", cli.Version, "config_path", configPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t := tray.New(configPath, logDir, cancel)
	trayDone := make(chan error, 1)
	go func() { trayDone <- t.Run(ctx) }()

	_, createErr := config.CreateDefault(configPath)
	runtimeConfig, configErr := config.LoadRuntime(configPath)
	if createErr != nil {
		configErr = createErr
	}
	var application *app.Application
	if configErr != nil {
		logger.Error("configuration error", "error", configErr)
		t.SetState(app.StateConfigurationError)
	} else {
		_ = logCloser.Close()
		logger, logCloser = logging.New(runtimeConfig.LogDirectory, runtimeConfig.LogLevel)
		backendClient := openssh.New(runtimeConfig.BackendPipePath, runtimeConfig.ConnectTimeout)
		var components []interfaces.Component
		if runtimeConfig.PageantEnabled {
			components = append(components, pageant.New(backendClient, logger))
		}
		if runtimeConfig.CygwinEnabled {
			components = append(components, cygwin.New(backendClient, runtimeConfig.CygwinPath, runtimeConfig.ConnectTimeout, logger))
		}
		application = app.New(components, t, logger)
		application.Run(ctx)
	}

	select {
	case err := <-trayDone:
		if err != nil {
			logger.Error("tray stopped unexpectedly", "error", err)
			return cli.ExitWindows
		}
	case <-ctx.Done():
	}
	cancel()
	if application != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := application.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown timeout", "error", err)
		}
	}
	select {
	case <-trayDone:
	case <-time.After(10 * time.Second):
		logger.Error("tray shutdown timeout")
	}
	logger.Info("application stopped", "executable", filepath.Base(os.Args[0]))
	return cli.ExitOK
}
