package telegram

import (
	"fmt"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	toolbox_lifecycle "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/lifecycle"
	toolbox_teleclient "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/teleremote"
	unilog_ifaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

// MenuManager orchestrates the rebuild operations of the Telegram interactive menus.
type MenuManager struct {
	tc         *toolbox_teleclient.TeleClient
	controller notif_core.NotifController
	logger     unilog_ifaces.Logger
}

// NewMenuManager creates a new MenuManager.
func NewMenuManager(tc *toolbox_teleclient.TeleClient, controller notif_core.NotifController, logger unilog_ifaces.Logger) *MenuManager {
	return &MenuManager{
		tc:         tc,
		controller: controller,
		logger:     logger,
	}
}

// RebuildMenu dynamically pulls the configuration map and registers it with the TeleClient.
func (m *MenuManager) RebuildMenu() {
	var actions []toolbox_teleclient.Action

	// 1. Actions Menu
	actions = append(actions, toolbox_teleclient.Action{
		Label: "🛠 Actions",
		SubMenu: []toolbox_teleclient.Action{
			{
				Label:       "✉️ Send Test Notification",
				InputPrompt: "Enter the message to send as an INFO test notification:",
				Callback: func(input string) error {
					if input == "" {
						return nil
					}
					return m.controller.SendTestNotification("INFO", "Tele-Remote Test", input)
				},
			},
			{
				Label: "🔄 Reload Senders",
				Callback: func(input string) error {
					return m.controller.ReloadConfig()
				},
			},
		},
	})

	// 2. Alerting Config Browser (Add/Edit/Remove)
	config := m.controller.GetAlertingConfig()
	var sections []toolbox_teleclient.Action

	// A. Manage Existing Providers
	for platform, settings := range config {
		tag := platform
		var keyActions []toolbox_teleclient.Action

		// List Keys for Editing
		for key, val := range settings {
			kName := key
			vVal := val
			keyActions = append(keyActions, toolbox_teleclient.Action{
				Label: fmt.Sprintf("🔑 %s", kName),
				SubMenu: []toolbox_teleclient.Action{
					{
						Label:    fmt.Sprintf("Value: %s", vVal),
						Callback: func(string) error { return nil },
					},
					{
						Label:       fmt.Sprintf("✏️ Edit %s", kName),
						InputPrompt: fmt.Sprintf("Enter new value for [%s] %s:", tag, kName),
						Callback: func(input string) error {
							if input == "" {
								return nil
							}
							err := m.controller.SetAlertingConfig(tag, kName, input)
							if err == nil {
								m.RebuildMenu()
							}
							return err
						},
					},
				},
			})
		}

		// Add "Delete Provider" Button
		keyActions = append(keyActions, toolbox_teleclient.Action{
			Label: fmt.Sprintf("🗑 Delete %s", tag),
			Callback: func(input string) error {
				err := m.controller.RemoveProvider(tag)
				if err == nil {
					m.RebuildMenu()
				}
				return err
			},
		})

		sections = append(sections, toolbox_teleclient.Action{
			Label:   fmt.Sprintf("📁 %s", tag),
			SubMenu: keyActions,
		})
	}

	// B. Add New Provider Logic
	types := m.controller.GetSupportedTypes()
	var addActions []toolbox_teleclient.Action
	for _, t := range types {
		platType := t
		addActions = append(addActions, toolbox_teleclient.Action{
			Label:       fmt.Sprintf("➕ New %s", platType),
			InputPrompt: fmt.Sprintf("Enter a UNIQUE TAG name for this %s instance:", platType),
			Callback: func(input string) error {
				if input == "" {
					return nil
				}
				err := m.controller.AddProvider(input, platType)
				if err == nil {
					m.RebuildMenu()
				}
				return err
			},
		})
	}
	sections = append(sections, toolbox_teleclient.Action{
		Label:   "➕ Add Provider",
		SubMenu: addActions,
	})

	actions = append(actions, toolbox_teleclient.Action{
		Label:   "🔍 Alerting Configuration",
		SubMenu: sections,
	})

	// 3. Status
	actions = append(actions, toolbox_teleclient.Action{
		Label: "📊 Status",
		Callback: func(input string) error {
			details, err := m.controller.GetStatus()
			if err != nil {
				return err
			}
			msg := fmt.Sprintf("Notif Server Status: Operational\nActive Notifiers: %s\nVersion: 0.1.1",
				details["active_notifiers"])
			return m.tc.SendTelemetry(msg)
		},
	})

	m.tc.UpdateActions(actions)
	m.tc.PushMenuUpdate()
}

// SetupTelegram initializes the Tele-Remote client, binds dynamic updates, and registers with Lifecycle Manager.
func SetupTelegram(appConfig *toolbox_config.AppConfig, controller notif_core.NotifController, logger unilog_ifaces.Logger, onUpdateRegistry func(func()), lm *toolbox_lifecycle.Manager) {
	var teleCap struct {
		IP   string `json:"ip"`
		Port string `json:"port"`
	}
	if err := appConfig.GetCapability("tele_remote", &teleCap); err != nil {
		logger.Warning("Tele-Remote capability not found or configured: %v", err)
		return
	}
	port := 50051
	if teleCap.Port != "" {
		fmt.Sscanf(teleCap.Port, "%d", &port)
	}

	ip := "127.0.0.1"
	if teleCap.IP != "" {
		ip = teleCap.IP
	}

	teleClient := toolbox_teleclient.NewTeleClient("Notif Server", ip, port, logger)
	mgr := NewMenuManager(teleClient, controller, logger)

	// Initial menu build
	mgr.RebuildMenu()

	// Register updater callback for any config updates
	onUpdateRegistry(func() {
		mgr.RebuildMenu()
	})

	teleClient.Start()

	lm.Register("TeleClient", func() error {
		teleClient.Close()
		return nil
	})
}
