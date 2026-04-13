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
	fmt.Printf("Starting Notif Server...\n")

	appConfig, err := utilconf.LoadConfig("test", nil)
	if err != nil {
		fmt.Printf("Critical Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 1. & 2. Initialize using Universal Logger
	// bootstrap.Init returns (*config.DistConfig, interfaces.Logger)
	rawConfig, uniLog := bootstrap.Init("notif-server", "test", "minimal", utils.LevelInfo, false)

	// Create Notifie
	notifObject := notifie.NewNotifie(rawConfig.Config, "notif-server")
	uniLog.Info("Notifie '%s' initialized", notifObject.Name)

	// 3. Bind local notifier
	uniLog.SetLocalNotifQueue(notifObject.NotifChan)

	// 4. Start Notification Server
	srv := server.NewServer(appConfig, uniLog, notifObject)

	go func() {
		if err := srv.Start(); err != nil {
			uniLog.Critical("Server failed: %v", err)
		}
	}()

	// 5. Graceful Shutdown via Toolbox
	lm := lifecycle.NewManager()
	lm.Register("NotificationServer", func() error {
		uniLog.Info("Shutting down Notification Server...")
		srv.Stop()
		return nil
	})

	lm.Wait(context.Background())
}
