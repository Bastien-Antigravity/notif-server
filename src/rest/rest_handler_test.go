package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	notif_core "github.com/Bastien-Antigravity/notif-server/src/core"
	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type mockNotifLogger struct{}

func (m *mockNotifLogger) Debug(format string, args ...any)                            {}
func (m *mockNotifLogger) Info(format string, args ...any)                             {}
func (m *mockNotifLogger) Warning(format string, args ...any)                          {}
func (m *mockNotifLogger) Error(format string, args ...any)                            {}
func (m *mockNotifLogger) Critical(format string, args ...any)                         {}
func (m *mockNotifLogger) Stream(format string, args ...any)                           {}
func (m *mockNotifLogger) Logon(format string, args ...any)                            {}
func (m *mockNotifLogger) Logout(format string, args ...any)                           {}
func (m *mockNotifLogger) Trade(format string, args ...any)                            {}
func (m *mockNotifLogger) Schedule(format string, args ...any)                         {}
func (m *mockNotifLogger) Report(format string, args ...any)                           {}
func (m *mockNotifLogger) GetNotifQueue() <-chan *unilog_interfaces.NotifMessage       { return nil }
func (m *mockNotifLogger) SetLocalNotifQueue(ch chan *unilog_interfaces.NotifMessage)  {}
func (m *mockNotifLogger) Log(level unilog_interfaces.Level, format string, args ...any) {}
func (m *mockNotifLogger) LogWithCaller(level unilog_interfaces.Level, msg, file, line, function, module string) {}
func (m *mockNotifLogger) SetLevel(level unilog_interfaces.Level)                      {}
func (m *mockNotifLogger) GetLevel() unilog_interfaces.Level                           { return unilog_interfaces.LevelInfo }
func (m *mockNotifLogger) SetCallerSkip(skip int)                                      {}
func (m *mockNotifLogger) SetMetadata(metadata map[string]string)                      {}
func (m *mockNotifLogger) GetMetadata() map[string]string                              { return nil }
func (m *mockNotifLogger) AddMetadata(key, value string)                               {}
func (m *mockNotifLogger) Close()                                                      {}

type mockNotifControl struct{}

func (m *mockNotifControl) ListNotifiers() []notif_core.NotifierInfo {
	return []notif_core.NotifierInfo{
		{Name: "telegram_ops", Type: "telegram", Healthy: true},
	}
}
func (m *mockNotifControl) SendTestNotification(level, title, message string) error { return nil }
func (m *mockNotifControl) ReloadConfig() error                                     { return nil }
func (m *mockNotifControl) GetStatus() (map[string]string, error) {
	return map[string]string{"workers": "1"}, nil
}
func (m *mockNotifControl) GetAlertingConfig() map[string]map[string]string { return nil }
func (m *mockNotifControl) SetAlertingConfig(platform, key, value string) error {
	return nil
}
func (m *mockNotifControl) AddProvider(tag, pType string) error { return nil }
func (m *mockNotifControl) RemoveProvider(tag string) error     { return nil }
func (m *mockNotifControl) GetSupportedTypes() []string         { return []string{"telegram", "discord", "gmail", "matrix"} }

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
