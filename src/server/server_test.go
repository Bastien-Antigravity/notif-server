package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"
	factory "github.com/Bastien-Antigravity/safe-socket"
	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	"github.com/Bastien-Antigravity/universal-logger/src/logger"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
	"github.com/stretchr/testify/assert"
)

// mockLogger implements the underlying logger interface for UniLog
type mockLogger struct {
	CapturedLogs []string
}

func (m *mockLogger) Debug(format string, args ...any) { fmt.Printf("DEBUG: "+format+"\n", args...) }
func (m *mockLogger) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	m.CapturedLogs = append(m.CapturedLogs, msg)
	fmt.Printf("INFO: "+format+"\n", args...)
}
func (m *mockLogger) Warning(format string, args ...any) { fmt.Printf("WARN: "+format+"\n", args...) }
func (m *mockLogger) Error(format string, args ...any)   { fmt.Printf("ERROR: "+format+"\n", args...) }
func (m *mockLogger) Critical(format string, args ...any) {
	fmt.Printf("CRITICAL: "+format+"\n", args...)
}
func (m *mockLogger) Logon(format string, args ...any)  { fmt.Printf("LOGON: "+format+"\n", args...) }
func (m *mockLogger) Logout(format string, args ...any) { fmt.Printf("LOGOUT: "+format+"\n", args...) }
func (m *mockLogger) Trade(format string, args ...any)  { fmt.Printf("TRADE: "+format+"\n", args...) }
func (m *mockLogger) Schedule(format string, args ...any) {
	fmt.Printf("SCHEDULE: "+format+"\n", args...)
}
func (m *mockLogger) Report(format string, args ...any) { fmt.Printf("REPORT: "+format+"\n", args...) }
func (m *mockLogger) Stream(format string, args ...any) { fmt.Printf("STREAM: "+format+"\n", args...) }
func (m *mockLogger) SetLevel(level utils.Level)        {}
func (m *mockLogger) Close()                            {}

// Required by UniLog
func (m *mockLogger) Log(lvl utils.Level, format string, args ...any) {}

func TestServerConnection(t *testing.T) {
	// 1. Setup config for a test server
	conf := distributed_config.New("test")
	// Use port 9999 for integration test
	conf.Capabilities["notif_server"] = map[string]interface{}{"ip": "127.0.0.1", "port": "9999"}

	// 2. Initialize dependencies
	ml := &mockLogger{}
	ul := logger.NewUniLog(ml)
	nt := notifie.NewNotifie(conf, "TestServer")
	srv := NewServer(conf, ul, nt)

	// 3. Start server in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- srv.Start()
	}()

	// Give the server a moment to start listening
	time.Sleep(500 * time.Millisecond)

	// 4. Act as a client and connect
	client, err := factory.Create("tcp-hello", "127.0.0.1:9999", "127.0.0.1", "client", true)
	if err != nil {
		t.Fatalf("Failed to connect to test server: %v", err)
	}
	defer client.Close()

	// 5. Assertions
	assert.NotNil(t, client, "Client should be connected")
	
	found := false
	for _, l := range ml.CapturedLogs {
		if contains := fmt.Sprintf("listening on 127.0.0.1:9999"); contains != "" {
			if strings.Contains(l, "listening on 127.0.0.1:9999") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Logger should have recorded the TCP listening address")

	// 6. Shutdown server
	srv.Stop()

	select {
	case err := <-done:
		assert.NoError(t, err, "Server should have shut down gracefully")
	case <-time.After(2 * time.Second):
		t.Fatal("Server timed out during shutdown")
	}
}
