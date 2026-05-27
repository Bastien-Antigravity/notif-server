package interfaces

/*
ESSENTIAL PROCESS:
Defines the standard interface for notification senders (e.g., Telegram, Discord).
Ensures that all notification platforms implement a unified dispatch method.

DATA FLOW:
1. Core dispatcher receives a notification.
2. Identifies the target sender implementation.
3. Calls SendMessage with context and payload.

KEY PARAMETERS:
- ctx: Execution context with timeout.
- msg: The primary notification text.
- to: Target recipient or room (platform-specific).
- subject: Optional metadata or header.
*/

import (
	"context"
)

type INotifSender interface {
	SendMessage(ctx context.Context, msg, to, subject string) error
	GetTag() string
	GetLogLevel() string
}
