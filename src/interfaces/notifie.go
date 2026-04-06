package interfaces

import "github.com/Bastien-Antigravity/universal-logger/src/utils"

// Notificator defines the interface for a notification service.
type Notificator interface {
	// Send sends a structured notification message.
	Send(msg *utils.NotifMessage)

	// SendRaw sends a raw byte message (serialized).
	SendRaw(data []byte)

	// LoadNotifSender loads sender configurations and returns a map of log levels to tags.
	LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string
}
