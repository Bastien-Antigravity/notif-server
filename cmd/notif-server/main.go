package main

/*
ESSENTIAL PROCESS:
Application entry point for the notif-server.
Initializes configuration, logging, and the core notification engine.
Manages the server lifecycle and graceful shutdown.

DATA FLOW:
1. Load configuration from YAML/CLI.
2. Bootstrap universal-logger with injected config.
3. Initialize the Notifier core and worker pools.
4. Start TCP and gRPC listeners.
5. Wait for termination signals to trigger graceful shutdown.

KEY PARAMETERS:
- Profile: The configuration profile (e.g., standalone, production).
- lm: Lifecycle manager for graceful service termination.
*/

import (
	"context"
	"fmt"
	"os"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/server"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	toolbox_lifecycle "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/lifecycle"
	unilog "github.com/Bastien-Antigravity/universal-logger/src/bootstrap"
	unilog_config "github.com/Bastien-Antigravity/universal-logger/src/config"
	unilog_utils "github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// -----------------------------------------------------------------------------

func main() {
	appConfig, err := toolbox_config.LoadConfig("standalone", nil)
	if err != nil {
		fmt.Printf("Critical Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 1. & 2. Initialize using Universal Logger with Injection
	// We inject the toolbox-loaded config to avoid double initialization
	_, uniLog := unilog.InitWithOptions(unilog.BootstrapOptions{
		Name:             "notif-server",
		ConfigProfile:    appConfig.Profile,
		LoggerProfile:    "standard",
		InitialLogLevel:  unilog_utils.LevelInfo,
		UseLocalNotifier: true,
		ExistingConfig:   &unilog_config.DistConfig{Config: appConfig.Config},
	})
	defer uniLog.Close()

	// Inject the logger back into the appConfig so toolbox can use it
	appConfig.Logger = uniLog

	uniLog.Info("Starting Notif Server...")

	// Create Notifier with injected logger
	notifObject := notif_core.NewNotifier(appConfig.Config, uniLog, "notif-server")
	uniLog.Info("Notifier '%s' initialized", notifObject.Name)

	// 4. Start Notification Server
	srv := server.NewServer(appConfig, uniLog, notifObject)

	go func() {
		if err := srv.Start(); err != nil {
			uniLog.Critical("Server failed: %v", err)
		}
	}()

	// 5. Graceful Shutdown via Toolbox
	lm := toolbox_lifecycle.NewManagerWithLogger(uniLog)
	lm.Register("NotificationServer", func() error {
		uniLog.Info("Shutting down Notification Server...")
		srv.Stop()
		return nil
	})

	lm.Wait(context.Background())
}
