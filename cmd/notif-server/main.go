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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/rest"
	"github.com/Bastien-Antigravity/notif-server/src/server"
	"github.com/Bastien-Antigravity/notif-server/src/telegram"

	toolbox_bootstrap "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/bootstrap"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	toolbox_lifecycle "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/lifecycle"
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

// -----------------------------------------------------------------------------
// Main Entry Point
// -----------------------------------------------------------------------------

func main() {
	// 1. Initialize Service via Unified Ecosystem Bootstrapper
	appConfig, uniLog := toolbox_bootstrap.BootstrapService("notif-server", "store")
	defer uniLog.Close()

	uniLog.Info("Starting Notif Server...")

	// Pre-flight check: ensure Telegram token is working before starting the process (fail-fast)
	verifyTelegramConfig(appConfig, uniLog)

	// Create Notifier with injected logger and toolbox AppConfig
	notifObject := notif_core.NewNotifier(appConfig, uniLog, "notif-server")
	uniLog.Info("Notifier '%s' initialized", notifObject.Name)

	// Bind Universal Logger notification queue so messages logged through universal-logger route into Notifier (ecosystem standard)
	uniLog.SetLocalNotifQueue(notifObject.NotifChan)

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

	restHandler := rest.NewRESTHandler(controller, uniLog)
	go func() {
		if err := restHandler.StartServer(restAddr); err != nil {
			uniLog.Error("REST management server failed: %v", err)
		}
	}()

	// Register notif-server OpenMFE with web-interface dynamically (non-blocking, infinite retry)
	go func() {
		mfeUrl := fmt.Sprintf("http://%s/static/js/mfe-loader.js", restAddr)
		payload := fmt.Sprintf(`{
			"name": "notif-server",
			"tag": "notif-server-mfe",
			"url": "%s",
			"navTitle": "🔔 Notif Server",
			"layer": "L1",
			"category": "Base Fleet",
			"priority": 30,
			"icon": "fa-bell",
			"requiresAuth": false
		}`, mfeUrl)

		client := &http.Client{Timeout: 3 * time.Second}
		backoff := 2 * time.Second
		maxBackoff := 15 * time.Second

		for {
			webAddr, err := appConfig.GetListenAddr("web_interface")
			if err == nil && webAddr != "" {
				regUrl := fmt.Sprintf("http://%s/api/v1/register", webAddr)
				resp, postErr := client.Post(regUrl, "application/json", strings.NewReader(payload))
				if postErr == nil {
					resp.Body.Close()
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						uniLog.Info("Successfully registered notif-server OpenMFE with web-interface at %s", regUrl)
						return
					}
					uniLog.Warning("OpenMFE registration attempt failed with status %s; retrying in %v...", resp.Status, backoff)
				} else {
					uniLog.Warning("OpenMFE registration attempt failed (%v); retrying in %v...", postErr, backoff)
				}
			} else {
				uniLog.Warning("web_interface capability address unresolvable (%v); retrying in %v...", err, backoff)
			}

			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * 1.5)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
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
		if restHandler != nil {
			_ = restHandler.Stop(context.Background())
		}
		if notifObject != nil {
			notifObject.Stop()
		}
		return nil
	})

	lm.Wait(context.Background())
}

// -----------------------------------------------------------------------------

// verifyTelegramConfig tests and verifies the configured Telegram bot token against the Telegram Bot API.
// Fails fast if the token is missing, cannot be decrypted, or fails Telegram API verification.
func verifyTelegramConfig(appConfig *toolbox_config.AppConfig, logger log_interfaces.Logger) {
	var token, url string
	var notifCap struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := appConfig.GetCapability("notif_server", &notifCap); err == nil {
		token = notifCap.Token
		url = notifCap.URL
	}
	if token == "" {
		logger.Critical("❌ No Telegram bot token configured in capabilities.notif_server; failing fast")
		os.Exit(1)
	}
	if url == "" {
		url = "https://api.telegram.org"
	}

	plainToken, err := appConfig.DecryptSecret(token)
	if err != nil {
		logger.Critical("❌ Failed to decrypt Telegram bot token: %v; failing fast", err)
		os.Exit(1)
	}

	logger.Info("🔍 Verifying Telegram Bot Token...")
	ok, username, err := verifyTelegramToken(url, plainToken)
	if err != nil {
		logger.Warning("Telegram token verification connection error: %v", err)
		return
	}
	if !ok {
		logger.Critical("❌ Telegram API verification failed: %s; failing fast", username)
		os.Exit(1)
	}
	logger.Info("✅ Telegram Bot Token verified successfully (%s)!", username)
}

// -----------------------------------------------------------------------------

// verifyTelegramToken validates a Telegram bot token against the getMe endpoint.
func verifyTelegramToken(baseURL, token string) (bool, string, error) {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return false, "", fmt.Errorf("telegram bot token cannot be empty")
	}

	apiURL := fmt.Sprintf("%s/bot%s/getMe", strings.TrimRight(baseURL, "/"), trimmedToken)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Bastien-Antigravity/NotifServer")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("failed to read response: %w", err)
	}

	var res struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return false, "", fmt.Errorf("failed to parse response (HTTP %d): %s", resp.StatusCode, string(body))
	}

	if res.OK {
		return true, fmt.Sprintf("@%s", res.Result.Username), nil
	}

	return false, res.Description, nil
}


