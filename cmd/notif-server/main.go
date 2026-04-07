package main

import (
	"fmt"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/server"

	"github.com/Bastien-Antigravity/universal-logger/src/bootstrap"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

func main() {
	fmt.Printf("Starting Notif Server...\n")

	// 1. & 2. Initialize using Universal Logger
	uniConfig, uniLog := bootstrap.Init("Notif-Server", "test", "minimal", utils.LevelInfo, false)

	// Create Notifie
	notifObject := notifie.NewNotifie(uniConfig, "NotifServer")
	uniLog.Info("Notifie '%s' initialized", notifObject.Name)

	// 3. Bind local notifier
	uniLog.SetLocalNotifQueue(notifObject.NotifChan)

	// 4. Start Notification Server
	srv := server.NewServer(uniConfig, uniLog, notifObject)

	err := srv.Start()
	if err != nil {
		fmt.Printf("Server failed to start/listen: %v\n", err)
	}
}
