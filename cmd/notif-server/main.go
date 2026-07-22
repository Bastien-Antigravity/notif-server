package main

/*
ESSENTIAL PROCESS:
Application entry point for the notif-server.
Initializes configuration, logging, and the core notification engine.
Manages the server lifecycle and graceful shutdown.

DATA FLOW:
1. Load config -> Initialize Universal Logger.
2. Bootstrap Notifier core with handlers.
3. Start TCP Ingestion Server and Management Interfaces.
4. Integrate with Tele-Remote for Telegram control.
*/

// [SCAN] Role: Developer | Source: main.go | State: Active

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/rest"
	"github.com/Bastien-Antigravity/notif-server/src/server"
	"github.com/Bastien-Antigravity/notif-server/src/telegram"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	toolbox_lifecycle "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/lifecycle"
	unilog "github.com/Bastien-Antigravity/universal-logger/src/bootstrap"
	unilog_config "github.com/Bastien-Antigravity/universal-logger/src/config"
	unilog_utils "github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// -----------------------------------------------------------------------------
// Main Entry Point
// -----------------------------------------------------------------------------

func main() {
	// 1. Initialize Toolbox Config (handles --profile automatically)
	appConfig, err := toolbox_config.LoadConfig("standalone", []string{"store"})
	if err != nil {
		fmt.Printf("Critical Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Logger via Universal Logger Bootstrapper
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

	// Create Controller Abstraction
	controller := notif_core.NewController(notifObject)

	// 4. Start Notification Server
	srv := server.NewServer(appConfig, uniLog, notifObject, controller)

	// 5. Start Management Interfaces (REST)
	restAddr, err := appConfig.GetRESTAddr("notif_server")
	if err != nil {
		uniLog.Critical("Failed to resolve REST address for notif_server: %v", err)
		os.Exit(1)
	}
	_, restPortStr, err := net.SplitHostPort(restAddr)
	if err != nil {
		uniLog.Critical("Failed to parse REST address '%s': %v", restAddr, err)
		os.Exit(1)
	}
	var restPort int
	if _, err := fmt.Sscanf(restPortStr, "%d", &restPort); err != nil {
		uniLog.Critical("Invalid REST port '%s': %v", restPortStr, err)
		os.Exit(1)
	}

	restHandler := rest.NewRESTHandler(controller, uniLog)
	go func() {
		if err := restHandler.StartServer(restPort); err != nil {
			uniLog.Error("REST management server failed: %v", err)
		}
	}()

	// Register notif-server OpenMFE with web-interface dynamically
	go func() {
		webAddr, err := appConfig.GetListenAddr("web_interface")
		if err != nil {
			uniLog.Warning("Could not resolve web_interface address for OpenMFE registration: %v", err)
			webAddr = "127.0.0.1:8080"
		}
		regUrl := fmt.Sprintf("http://%s/api/v1/register", webAddr)

		mfeUrl := fmt.Sprintf("http://%s/static/js/mfe-loader.js", restAddr)

		payload := fmt.Sprintf(`{
			"name": "notif-server",
			"tag": "notif-server-mfe",
			"url": "%s",
			"navTitle": "🔔 Notif Server"
		}`, mfeUrl)

		client := &http.Client{Timeout: 3 * time.Second}
		for i := 0; i < 15; i++ {
			resp, err := client.Post(regUrl, "application/json", strings.NewReader(payload))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					uniLog.Info("Successfully registered notif-server OpenMFE with web-interface at %s", regUrl)
					return
				}
				uniLog.Warning("OpenMFE registration attempt %d failed with status %s", i+1, resp.Status)
			} else {
				uniLog.Warning("OpenMFE registration attempt %d failed: %v", i+1, err)
			}
			time.Sleep(3 * time.Second)
		}
		uniLog.Warning("Failed to register notif-server OpenMFE with web-interface after 15 attempts")
	}()

	go func() {
		if err := srv.Start(); err != nil {
			uniLog.Critical("Server failed: %v", err)
		}
	}()

	// 6. Graceful Shutdown via Toolbox
	lm := toolbox_lifecycle.NewManagerWithLogger(uniLog)

	telegram.SetupTelegram(appConfig, controller, uniLog, func(cb func()) {
		notifObject.OnUpdate = cb
	}, lm)

	lm.Register("NotificationServer", func() error {
		uniLog.Info("Shutting down Notification Server...")
		srv.Stop()
		return nil
	})

	lm.Wait(context.Background())
}
