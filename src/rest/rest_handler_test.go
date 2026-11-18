package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type mockNotifLogger struct {
	infoLogs []string
}

func (m *mockNotifLogger) Debug(format string, args ...any) {}
func (m *mockNotifLogger) Info(format string, args ...any) {
	m.infoLogs = append(m.infoLogs, fmt.Sprintf(format, args...))
}
func (m *mockNotifLogger) Warning(format string, args ...any)                            {}
func (m *mockNotifLogger) Error(format string, args ...any)                              {}
func (m *mockNotifLogger) Critical(format string, args ...any)                           {}
func (m *mockNotifLogger) Stream(format string, args ...any)                             {}
func (m *mockNotifLogger) Logon(format string, args ...any)                              {}
func (m *mockNotifLogger) Logout(format string, args ...any)                             {}
func (m *mockNotifLogger) Trade(format string, args ...any)                              {}
func (m *mockNotifLogger) Schedule(format string, args ...any)                           {}
func (m *mockNotifLogger) Report(format string, args ...any)                             {}
func (m *mockNotifLogger) GetNotifQueue() <-chan *unilog_interfaces.NotifMessage         { return nil }
func (m *mockNotifLogger) SetLocalNotifQueue(ch chan *unilog_interfaces.NotifMessage)    {}
func (m *mockNotifLogger) Log(level unilog_interfaces.Level, format string, args ...any) {}
func (m *mockNotifLogger) LogWithCaller(level unilog_interfaces.Level, msg, file, line, function, module string) {
}
func (m *mockNotifLogger) SetLevel(level unilog_interfaces.Level) {}
func (m *mockNotifLogger) GetLevel() unilog_interfaces.Level      { return unilog_interfaces.LevelInfo }
func (m *mockNotifLogger) SetCallerSkip(skip int)                 {}
func (m *mockNotifLogger) SetMetadata(metadata map[string]string) {}
func (m *mockNotifLogger) GetMetadata() map[string]string         { return nil }
func (m *mockNotifLogger) AddMetadata(key, value string)          {}
func (m *mockNotifLogger) Close()                                 {}

type mockNotifControl struct {
	config         map[string]map[string]string
	lastTargetedTo string
}

func (m *mockNotifControl) ListNotifiers() []notif_core.NotifierInfo {
	return []notif_core.NotifierInfo{
		{Name: "telegram_ops", Type: "telegram", Healthy: true},
	}
}
func (m *mockNotifControl) SendTestNotification(level, title, message string) error { return nil }
func (m *mockNotifControl) SendTestNotificationToTag(tag, level, title, message string) error {
	m.lastTargetedTo = tag
	return nil
}
func (m *mockNotifControl) ReloadConfig() error { return nil }
func (m *mockNotifControl) GetStatus() (map[string]string, error) {
	return map[string]string{"workers": "1"}, nil
}
func (m *mockNotifControl) GetAlertingConfig() map[string]map[string]string { return m.config }
func (m *mockNotifControl) SetAlertingConfig(platform, key, value string) error {
	if m.config == nil {
		m.config = make(map[string]map[string]string)
	}
	if m.config[platform] == nil {
		m.config[platform] = make(map[string]string)
	}
	m.config[platform][key] = value
	return nil
}
func (m *mockNotifControl) AddProvider(tag, pType string) error { return nil }
func (m *mockNotifControl) RemoveProvider(tag string) error     { return nil }
func (m *mockNotifControl) GetSupportedTypes() []string {
	return []string{"telegram", "discord", "gmail", "matrix"}
}

func TestNotifHandlerContractsEndpoint(t *testing.T) {
	h := NewRESTHandler(&mockNotifControl{}, &mockNotifLogger{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	rr := httptest.NewRecorder()
	h.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/contracts, got %d", rr.Code)
	}

	var env map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode contracts JSON: %v", err)
	}
	if env["service"] != "notif_server" || env["reliable"] != true || env["exhaustive"] != true {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	contracts, _ := env["contracts"].([]interface{})
	if len(contracts) < 4 {
		t.Fatalf("expected >=4 contract types, got %d", len(contracts))
	}
}

func TestGetAlertingConfigMasksAllSecretsForWebInterface(t *testing.T) {
	ctrl := &mockNotifControl{
		config: map[string]map[string]string{
			"telegram": {
				"TYPE":   "TELEGRAM",
				"TOKEN":  "123456:SUPER_SECRET_BOT_TOKEN",
				"CHATID": "ENC(encrypted_chat_id)",
				"URL":    "https://api.telegram.org",
			},
			"discord_ops": {
				"TYPE":     "DISCORD",
				"URL":      "https://discord.com/api/webhooks/111/DISCORD_WEBHOOK_SECRET",
				"LOGLEVEL": "ERROR",
			},
			"matrix_ops": {
				"TYPE": "MATRIX",
				"URL":  "https://matrix.example.com/hook?token=MATRIX_SECRET_TOKEN",
			},
			"gmail_ops": {
				"TYPE":           "GMAIL",
				"FROM":           "alerts@example.com",
				"PASSWD":         "GMAIL_APP_SECRET_PASS",
				"ATTACHMENT_DIR": "/tmp/notif-attachments",
			},
		},
	}

	h := NewRESTHandler(ctrl, &mockNotifLogger{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/alerting/get", nil)
	rr := httptest.NewRecorder()
	h.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()
	// Verify zero secret leakage in HTTP JSON response sent to web-interface
	for _, secret := range []string{
		"SUPER_SECRET_BOT_TOKEN",
		"encrypted_chat_id",
		"ENC(",
		"DISCORD_WEBHOOK_SECRET",
		"MATRIX_SECRET_TOKEN",
		"GMAIL_APP_SECRET_PASS",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("secret %q leaked in /api/v1/config/alerting/get response: %s", secret, body)
		}
	}

	// Non-secret config must remain visible
	for _, visible := range []string{
		"https://api.telegram.org",
		"alerts@example.com",
		"/tmp/notif-attachments",
		"********",
	} {
		if !strings.Contains(body, visible) {
			t.Fatalf("expected non-secret value %q in response: %s", visible, body)
		}
	}
}

func TestSetAlertingConfigRejectsMaskedPlaceholderAndRedactsLogs(t *testing.T) {
	logger := &mockNotifLogger{}
	ctrl := &mockNotifControl{
		config: map[string]map[string]string{
			"discord_ops": {
				"TYPE": "DISCORD",
				"URL":  "https://discord.com/api/webhooks/111/ORIGINAL_SECRET",
			},
		},
	}

	h := NewRESTHandler(ctrl, logger)

	// 1. Submitting "********" must be rejected without overwriting real secret
	badBody := bytes.NewBufferString(`{"platform":"discord_ops","key":"URL","value":"********"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/alerting/set", badBody)
	rr := httptest.NewRecorder()
	h.Handler().ServeHTTP(rr, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] == true || resp["error_code"] != "MASKED_VALUE_REJECTED" {
		t.Fatalf("expected MASKED_VALUE_REJECTED, got: %+v", resp)
	}
	if ctrl.config["discord_ops"]["URL"] != "https://discord.com/api/webhooks/111/ORIGINAL_SECRET" {
		t.Fatalf("original secret was overwritten!")
	}

	// 2. Submitting a new secret must succeed while redacting the secret from logs
	goodBody := bytes.NewBufferString(`{"platform":"discord_ops","key":"URL","value":"https://discord.com/api/webhooks/222/BRAND_NEW_SECRET"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/config/alerting/set", goodBody)
	rr2 := httptest.NewRecorder()
	h.Handler().ServeHTTP(rr2, req2)

	for _, logLine := range logger.infoLogs {
		if strings.Contains(logLine, "BRAND_NEW_SECRET") {
			t.Fatalf("secret leaked into SetAlertingConfig info log: %s", logLine)
		}
	}
}

func TestSendTestNotificationTargetedProvider(t *testing.T) {
	ctrl := &mockNotifControl{}
	h := NewRESTHandler(ctrl, &mockNotifLogger{})

	body := bytes.NewBufferString(`{"tag":"discord_ops","level":"INFO","title":"Self-Test","message":"Ping"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notif/test", body)
	rr := httptest.NewRecorder()
	h.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
	if ctrl.lastTargetedTo != "discord_ops" {
		t.Fatalf("expected targeted test to route to discord_ops, got %q", ctrl.lastTargetedTo)
	}
}
