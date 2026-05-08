package main

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
