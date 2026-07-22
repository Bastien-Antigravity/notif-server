package notifier

import (
	"fmt"
	"strings"

	unilog_utils "github.com/Bastien-Antigravity/universal-logger/src/utils"
)

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

// NewController creates a new Controller instance.
func NewController(n *Notifier) *Controller {
	return &Controller{notifier: n}
}

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

// SendTestNotification triggers a test notification through the engine.
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

	// Push to engine
	c.notifier.NotifChan <- msg
	return nil
}

// ReloadConfig triggers a manual reload of the configuration.
func (c *Controller) ReloadConfig() error {
	return c.notifier.GetConfig().Reload()
}

// GetStatus returns server health and metadata.
func (c *Controller) GetStatus() (map[string]string, error) {
	details := make(map[string]string)
	notifiers := c.notifier.GetActiveNotifiers()
	details["active_notifiers"] = fmt.Sprintf("%d", len(notifiers))
	return details, nil
}

// GetAlertingConfig returns the current internal configuration of notification providers.
func (c *Controller) GetAlertingConfig() map[string]map[string]string {
	c.notifier.mu.RLock()
	defer c.notifier.mu.RUnlock()
	return c.notifier.currentConf
}

// SetAlertingConfig updates a specific setting for a notification provider.
func (c *Controller) SetAlertingConfig(tag, key, value string) error {
	c.notifier.mu.Lock()
	if _, ok := c.notifier.currentConf[tag]; !ok {
		c.notifier.currentConf[tag] = make(map[string]string)
	}
	c.notifier.currentConf[tag][key] = value
	newConf := c.notifier.currentConf
	c.notifier.mu.Unlock()

	// Trigger reload to apply changes
	c.notifier.Reload(newConf)
	return nil
}

// AddProvider initializes a new provider section with default template.
func (c *Controller) AddProvider(tag, platType string) error {
	c.notifier.mu.Lock()
	if _, ok := c.notifier.currentConf[tag]; ok {
		c.notifier.mu.Unlock()
		return fmt.Errorf("provider with tag '%s' already exists", tag)
	}

	pType := strings.ToUpper(platType)
	template := map[string]string{
		"TYPE":     pType,
		"LOGLEVEL": "INFO",
		"TAG":      tag,
	}

	switch pType {
	case "TELEGRAM":
		template["TOKEN"] = "CHANGEME"
		template["CHATID"] = "CHANGEME"
		template["URL"] = "https://api.telegram.org"
	case "DISCORD":
		template["URL"] = "CHANGEME" // Matches discordUrl logic
	case "MATRIX":
		template["HOMESERVER"] = "CHANGEME"
		template["USER"] = "CHANGEME"
		template["PASSWORD"] = "CHANGEME"
		template["ROOM_ID"] = "CHANGEME"
	case "GMAIL":
		template["SMTP_SERVER"] = "CHANGEME"
		template["PORT"] = "587"
		template["USERNAME"] = "CHANGEME"
		template["PASSWORD"] = "CHANGEME"
		template["TO"] = "CHANGEME"
	}

	c.notifier.currentConf[tag] = template
	newConf := c.notifier.currentConf
	c.notifier.mu.Unlock()

	c.notifier.Reload(newConf)
	return nil
}

// RemoveProvider deletes a provider section.
func (c *Controller) RemoveProvider(tag string) error {
	c.notifier.mu.Lock()
	if _, ok := c.notifier.currentConf[tag]; !ok {
		c.notifier.mu.Unlock()
		return fmt.Errorf("provider with tag '%s' not found", tag)
	}

	delete(c.notifier.currentConf, tag)
	newConf := c.notifier.currentConf
	c.notifier.mu.Unlock()

	c.notifier.Reload(newConf)
	return nil
}

// GetSupportedTypes returns the list of hardcoded drivers available.
func (c *Controller) GetSupportedTypes() []string {
	return []string{"TELEGRAM", "DISCORD", "MATRIX", "GMAIL"}
}
