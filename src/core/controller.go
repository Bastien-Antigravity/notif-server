package notifier

/*
ESSENTIAL PROCESS:
Provides unified management, status reporting, and alerting configuration for notif-server.
Decouples external management interfaces (REST, gRPC, Tele-Remote) from the internal Notifier core.

DATA FLOW:
1. REST/gRPC/Tele-Remote calls Controller management methods.
2. Controller reads or deep-copies internal Notifier configuration.
3. Controller applies mutations to independent config copies and invokes Notifier.Reload.
4. Active sender states and worker pools are reloaded without mutating shared in-flight maps.

KEY PARAMETERS:
- Notifier: Pointer to the core Notifier engine.
- NotifierInfo: Public struct describing active sender tag, type, and health.
*/

import (
	"fmt"
	"strings"

	unilog_utils "github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// -----------------------------------------------------------------------------
// Models & Interfaces
// -----------------------------------------------------------------------------

// NotifierInfo provides a summary of an active notifier.
type NotifierInfo struct {
	Name    string
	Type    string
	Healthy bool
}

// NotifController defines the unified interface for notification management.
type NotifController interface {
	ListNotifiers() []NotifierInfo
	SendTestNotification(level, title, message string) error
	ReloadConfig() error
	GetStatus() (map[string]string, error)

	// Alerting Configuration
	GetAlertingConfig() map[string]map[string]string
	SetAlertingConfig(platform, key, value string) error
	AddProvider(tag, platType string) error
	RemoveProvider(tag string) error
	GetSupportedTypes() []string
}

// Controller implements the NotifController interface.
type Controller struct {
	notifier *Notifier
}

// -----------------------------------------------------------------------------

// NewController creates a new Controller instance.
func NewController(n *Notifier) *Controller {
	return &Controller{notifier: n}
}

// -----------------------------------------------------------------------------

// ListNotifiers returns the list of configured notifiers.
func (c *Controller) ListNotifiers() []NotifierInfo {
	notifiers := c.notifier.GetActiveNotifiers()
	res := make([]NotifierInfo, 0, len(notifiers))
	for _, n := range notifiers {
		res = append(res, NotifierInfo{
			Name:    n.Name,
			Type:    n.Type,
			Healthy: n.Healthy,
		})
	}
	return res
}

// -----------------------------------------------------------------------------

// SendTestNotification triggers a test notification through the engine and logger.
func (c *Controller) SendTestNotification(level, title, message string) error {
	lvl := unilog_utils.GetLogLevel(level)
	finalMsg := message
	if title != "" {
		finalMsg = fmt.Sprintf("[%s] %s", title, message)
	}

	msg := &unilog_utils.NotifMessage{
		Level:   lvl.String(),
		Message: finalMsg,
	}

	// Queue to engine channel
	select {
	case c.notifier.NotifChan <- msg:
	default:
		if c.notifier.Logger != nil {
			c.notifier.Logger.Error("NotifChan buffer full! Dropped test notification message: %s", finalMsg)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------

// ReloadConfig triggers a manual reload of the configuration.
func (c *Controller) ReloadConfig() error {
	return c.notifier.appConfig.Reload()
}

// -----------------------------------------------------------------------------

// GetStatus returns server health and metadata.
func (c *Controller) GetStatus() (map[string]string, error) {
	details := make(map[string]string)
	notifiers := c.notifier.GetActiveNotifiers()
	details["active_notifiers"] = fmt.Sprintf("%d", len(notifiers))
	return details, nil
}

// -----------------------------------------------------------------------------

// GetAlertingConfig returns a deep copy of the current internal configuration of notification providers.
func (c *Controller) GetAlertingConfig() map[string]map[string]string {
	c.notifier.mu.RLock()
	defer c.notifier.mu.RUnlock()
	return cloneConfig(c.notifier.currentConf)
}

// -----------------------------------------------------------------------------

// SetAlertingConfig updates a specific setting for a notification provider.
func (c *Controller) SetAlertingConfig(tag, key, value string) error {
	c.notifier.mu.Lock()
	newConf := cloneConfig(c.notifier.currentConf)
	if _, ok := newConf[tag]; !ok {
		newConf[tag] = make(map[string]string)
	}
	newConf[tag][key] = value
	c.notifier.mu.Unlock()

	// Trigger reload with the newly cloned configuration so change detection succeeds
	c.notifier.Reload(newConf)
	return nil
}

// -----------------------------------------------------------------------------

// AddProvider initializes a new provider section with default template.
func (c *Controller) AddProvider(tag, platType string) error {
	c.notifier.mu.Lock()
	if _, ok := c.notifier.currentConf[tag]; ok {
		c.notifier.mu.Unlock()
		return fmt.Errorf("provider with tag '%s' already exists", tag)
	}

	newConf := cloneConfig(c.notifier.currentConf)
	c.notifier.mu.Unlock()

	pType := strings.ToUpper(platType)
	template := map[string]string{
		"TYPE":     pType,
		"LOGLEVEL": "INFO",
		"TAG":      tag,
	}

	switch pType {
	case "TELEGRAM":
		template["TOKEN"] = ""
		template["CHATID"] = ""
		template["URL"] = "https://api.telegram.org"
	case "DISCORD":
		template["URL"] = ""
	case "MATRIX":
		template["URL"] = ""
	case "GMAIL":
		template["FROM"] = ""
		template["TO"] = ""
		template["PASSWD"] = ""
		template["SMTP_HOST"] = "smtp.gmail.com"
		template["SMTP_PORT"] = "587"
	}

	newConf[tag] = template
	c.notifier.Reload(newConf)
	return nil
}

// -----------------------------------------------------------------------------

// RemoveProvider deletes a provider section.
func (c *Controller) RemoveProvider(tag string) error {
	c.notifier.mu.Lock()
	if _, ok := c.notifier.currentConf[tag]; !ok {
		c.notifier.mu.Unlock()
		return fmt.Errorf("provider with tag '%s' not found", tag)
	}

	newConf := cloneConfig(c.notifier.currentConf)
	delete(newConf, tag)
	c.notifier.mu.Unlock()

	c.notifier.Reload(newConf)
	return nil
}

// -----------------------------------------------------------------------------

// GetSupportedTypes returns the list of hardcoded drivers available.
func (c *Controller) GetSupportedTypes() []string {
	return []string{"TELEGRAM", "DISCORD", "MATRIX", "GMAIL"}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func cloneConfig(original map[string]map[string]string) map[string]map[string]string {
	if original == nil {
		return make(map[string]map[string]string)
	}
	clone := make(map[string]map[string]string, len(original))
	for section, kv := range original {
		sectionClone := make(map[string]string, len(kv))
		for k, v := range kv {
			sectionClone[k] = v
		}
		clone[section] = sectionClone
	}
	return clone
}
