package notifier

/*
ESSENTIAL PROCESS:
Unit tests for NotifController implementation.
Validates dynamic provider addition, removal, configuration updates,
deep-copy immutability, and template generation for all supported provider types.

DATA FLOW:
1. Initialize Notifier with in-memory distributed config.
2. Invoke controller mutation methods (AddProvider, RemoveProvider, SetAlertingConfig).
3. Verify that Reload detects state transitions and updates internal maps.
4. Verify that mutations to external map references do not corrupt controller state.

KEY PARAMETERS:
- c: NotifController instance being tested.
- n: Underlying Notifier instance.
*/

import (
	"testing"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------

func TestControllerAddProviderDetectsChange(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, &testNotifierLogger{}, "ControllerTest")
	defer n.Stop()

	c := NewController(n)

	// Verify initial state is empty
	initialConfig := c.GetAlertingConfig()
	assert.Empty(t, initialConfig)

	// Add Telegram provider template
	err = c.AddProvider("tg-alert", "TELEGRAM")
	require.NoError(t, err)

	// Configure valid credentials
	err = c.SetAlertingConfig("tg-alert", "TOKEN", "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11")
	require.NoError(t, err)
	err = c.SetAlertingConfig("tg-alert", "CHATID", "987654321")
	require.NoError(t, err)

	// Verify provider was registered in alerting config
	currentConfig := c.GetAlertingConfig()
	assert.Contains(t, currentConfig, "tg-alert")
	assert.Equal(t, "TELEGRAM", currentConfig["tg-alert"]["TYPE"])
	assert.Equal(t, "987654321", currentConfig["tg-alert"]["CHATID"])
	assert.Equal(t, "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", currentConfig["tg-alert"]["TOKEN"])

	// Verify that provider is active in Notifier
	activeNotifiers := n.GetActiveNotifiers()
	found := false
	for _, notif := range activeNotifiers {
		if notif.Name == "tg-alert" {
			found = true
			break
		}
	}
	assert.True(t, found, "tg-alert should be an active notifier in worker pool")
}

// -----------------------------------------------------------------------------

func TestControllerRemoveProvider(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, &testNotifierLogger{}, "ControllerTest")
	defer n.Stop()

	c := NewController(n)

	// Add provider first
	err = c.AddProvider("tg-temp", "TELEGRAM")
	require.NoError(t, err)
	assert.Contains(t, c.GetAlertingConfig(), "tg-temp")

	// Remove provider
	err = c.RemoveProvider("tg-temp")
	require.NoError(t, err)

	// Verify removed from config and active notifiers
	assert.NotContains(t, c.GetAlertingConfig(), "tg-temp")
	for _, notif := range n.GetActiveNotifiers() {
		assert.NotEqual(t, "tg-temp", notif.Name, "tg-temp should no longer be active")
	}
}

// -----------------------------------------------------------------------------

func TestControllerDeepCopyImmutability(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, &testNotifierLogger{}, "ControllerTest")
	defer n.Stop()

	c := NewController(n)

	err = c.AddProvider("tg-immutable", "TELEGRAM")
	require.NoError(t, err)

	err = c.SetAlertingConfig("tg-immutable", "TOKEN", "initial-token")
	require.NoError(t, err)

	// Mutate the returned config map from GetAlertingConfig
	cfg := c.GetAlertingConfig()
	assert.Equal(t, "initial-token", cfg["tg-immutable"]["TOKEN"])
	cfg["tg-immutable"]["TOKEN"] = "mutated-after-get"
	cfg["new-provider"] = map[string]string{"TYPE": "DISCORD"}

	// Verify controller state was NOT corrupted by external mutations
	cfg2 := c.GetAlertingConfig()
	assert.Equal(t, "initial-token", cfg2["tg-immutable"]["TOKEN"])
	assert.NotContains(t, cfg2, "new-provider")
}

// -----------------------------------------------------------------------------

func TestControllerProviderTemplates(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, &testNotifierLogger{}, "ControllerTest")
	defer n.Stop()

	c := NewController(n)

	// Test template for MATRIX
	err = c.AddProvider("matrix-prod", "MATRIX")
	require.NoError(t, err)
	cfg := c.GetAlertingConfig()
	require.Contains(t, cfg, "matrix-prod")
	assert.Contains(t, cfg["matrix-prod"], "URL", "MATRIX template must supply URL parameter")

	// Test template for GMAIL
	err = c.AddProvider("gmail-prod", "GMAIL")
	require.NoError(t, err)
	cfg = c.GetAlertingConfig()
	require.Contains(t, cfg, "gmail-prod")
	gmailSettings := cfg["gmail-prod"]
	assert.Contains(t, gmailSettings, "FROM", "GMAIL template must supply FROM")
	assert.Contains(t, gmailSettings, "PASSWD", "GMAIL template must supply PASSWD")
	assert.Contains(t, gmailSettings, "SMTP_HOST", "GMAIL template must supply SMTP_HOST")
	assert.Contains(t, gmailSettings, "SMTP_PORT", "GMAIL template must supply SMTP_PORT")

	// Test duplicate provider error
	err = c.AddProvider("matrix-prod", "MATRIX")
	assert.Error(t, err, "Adding existing provider tag should return an error")
}
