package main

import (
	"fmt"

	notifie "notif-server/src/core"
	"notif-server/src/server"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	profiles "github.com/Bastien-Antigravity/flexible-logger/src/profiles"
)

const Version = "v1.0.0"

func main() {
	fmt.Printf("Starting Notif Server %s...\n", Version)

	// 1. Create Distributed Config
	distConf := distributed_config.New("test")

	// 2. High Performance Logger (Async everything)
	perfLog := profiles.NewNotifLogger("Notif-Server", distConf)

	// Create Notifie
	notifObject := notifie.NewNotifie(distConf, "NotifServer")
	perfLog.Info(fmt.Sprintf("Notifie '%s' initialized", notifObject.Name))

	// 3. Bind local notifier
	perfLog.SetLocalNotifQueue(notifObject.NotifChan)

	// 4. Start Notification Server
	srv := server.NewServer(distConf, perfLog, notifObject)

	err := srv.Start()
	if err != nil {
		fmt.Printf("Server failed to start/listen: %v\n", err)
	}
}
