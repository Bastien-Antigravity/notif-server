package notifier

/*
ESSENTIAL PROCESS:
Facade for the core notifier package.
Re-exports primary types and interfaces to provide a clean public API.

DATA FLOW:
1. Consumers import this package and use the aliased types.
2. Ensures internal structural changes do not break external consumers.

KEY PARAMETERS:
- Notifier: The core dispatching engine.
- NotifNcapHandler: Serialization handler for Cap'n Proto.
*/

import (
	"github.com/Bastien-Antigravity/notif-server/src/interfaces"
)

// Re-export interfaces for external consumers
type INotifier = interfaces.INotifier
type INotifSender = interfaces.INotifSender
type IConfigurableNotifier = interfaces.IConfigurableNotifier

// Notifier is already in this package, so no alias needed if used within the same package.
// However, if we wanted to expose it from a different root, we would alias it there.
