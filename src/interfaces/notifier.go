package interfaces

import "github.com/Bastien-Antigravity/universal-logger/src/utils"

// Notifier defines the interface for a notification service capable of sending messages.
type Notifier interface {
	// Notify sends a structured notification message.
	Notify(msg *utils.NotifMessage) error

	// SendRaw sends a raw byte message (serialized).
	SendRaw(data []byte) error
}

// ConfigurableNotifier defines a notifier that can load its own sender configuration.
type ConfigurableNotifier interface {
	Notifier
	// LoadNotifSender loads sender configurations and returns a map of log levels to tags.
	LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string
}
