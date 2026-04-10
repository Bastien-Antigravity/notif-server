package main

import (
	"context"
	"fmt"
	"os"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/server"

	utilconf "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/lifecycle"
	"github.com/Bastien-Antigravity/universal-logger/src/bootstrap"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

func main() {
	// 1. Initialize Toolbox Config
	appConfig, err := utilconf.LoadConfig("standalone", nil)
	if err != nil {
		fmt.Printf("Critical Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Logger
	_, appLogger := bootstrap.Init("notif-server", "standalone", "minimal", utils.LevelInfo, false)
	defer appLogger.Close()

	appLogger.Info("Bootstrapping Notif Server...")

	// Create Notifie
	notifObject := notifie.NewNotifie(appConfig.Config, "notif-server")
	appLogger.Info(fmt.Sprintf("Notifier '%s' initialized", notifObject.Name))

	// 3. Bind local notifier
	appLogger.SetLocalNotifQueue(notifObject.NotifChan)

	// 4. Start Notification Server
	srv := server.NewServer(appConfig.Config, appLogger, notifObject)

	go func() {
		if err := srv.Start(); err != nil {
			appLogger.Error(fmt.Sprintf("Server failed: %v", err))
		}
	}()

	// 5. Graceful Shutdown via Toolbox
	lm := lifecycle.NewManager()
	lm.Register("Cleanup", func() error {
		appLogger.Info("Shutting down notif-server...")
		return nil
	})

	lm.Wait(context.Background())
}
