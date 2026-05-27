package interfaces

/*
ESSENTIAL PROCESS:
Defines the core Notifier interfaces for the server.
Supports both structured and raw binary notification ingestion.

DATA FLOW:
1. External sources call Notify or SendRaw.
2. Ingestion layer queues the message for dispatch.

KEY PARAMETERS:
- msg: Structured notification message with tags and attachment.
- data: Raw binary payload (Cap'n Proto).
*/

import (
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// INotifier defines the interface for a notification service capable of sending messages.
type INotifier interface {
	// Notify sends a structured notification message.
	Notify(msg *utils.NotifMessage) error

	// SendRaw sends a raw byte message (serialized).
	SendRaw(data []byte) error
}

// IConfigurableNotifier defines a notifier that can load its own sender configuration.
type IConfigurableNotifier interface {
	INotifier
	// LoadNotifSender loads sender configurations and returns a map of log levels to tags.
	LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string
}
