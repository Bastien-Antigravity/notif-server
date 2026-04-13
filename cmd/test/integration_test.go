package main

import (
	"fmt"
	"testing"
	"time"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/notif-server/src/server"
	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/universal-logger/src/logger"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
	factory "github.com/Bastien-Antigravity/safe-socket"
	"github.com/stretchr/testify/assert"
)

// mockLogger implementation for the integration test
type mockLogger struct {
	lastMsg string
}

func (m *mockLogger) Debug(format string, args ...any)    { fmt.Printf("DEBUG: "+format+"\n", args...) }
func (m *mockLogger) Info(format string, args ...any)     { m.lastMsg = fmt.Sprintf(format, args...); fmt.Printf("INFO: "+format+"\n", args...) }
func (m *mockLogger) Warning(format string, args ...any)  { fmt.Printf("WARN: "+format+"\n", args...) }
func (m *mockLogger) Error(format string, args ...any)    { fmt.Printf("ERROR: "+format+"\n", args...) }
func (m *mockLogger) Critical(format string, args ...any) { fmt.Printf("CRITICAL: "+format+"\n", args...) }
func (m *mockLogger) Logon(format string, args ...any)    { fmt.Printf("LOGON: "+format+"\n", args...) }
func (m *mockLogger) Logout(format string, args ...any)   { fmt.Printf("LOGOUT: "+format+"\n", args...) }
func (m *mockLogger) Trade(format string, args ...any)    { fmt.Printf("TRADE: "+format+"\n", args...) }
func (m *mockLogger) Schedule(format string, args ...any) { fmt.Printf("SCHEDULE: "+format+"\n", args...) }
func (m *mockLogger) Report(format string, args ...any)   { fmt.Printf("REPORT: "+format+"\n", args...) }
func (m *mockLogger) Stream(format string, args ...any)   { fmt.Printf("STREAM: "+format+"\n", args...) }
func (m *mockLogger) SetLevel(level utils.Level)          {}
func (m *mockLogger) SetCallerSkip(skip int)           {}
func (m *mockLogger) Close()                              {}
func (m *mockLogger) Log(lvl utils.Level, format string, args ...any) {}

// mockSender to verify the final delivery
type mockSender struct {
	received chan string
}

func (m *mockSender) SendMessage(msg, to, subject string) error {
	m.received <- msg
	return nil
}

func TestE2EFlow(t *testing.T) {
	// 1. Setup Configuration
	conf := distributed_config.New("test")
	conf.Capabilities["notif_server"] = map[string]interface{}{"ip": "127.0.0.1", "port": "10001"}

	// 2. Initialize Components
	ml := &mockLogger{}
	ul := logger.NewUniLog(ml)
	nt := notifie.NewNotifie(conf, ul, "E2E-Integration")
	
	// Register a mock sender to capture the final output
	ms := &mockSender{received: make(chan string, 1)}
	nt.TagToSenderMap["alert"] = ms

	ac := &toolbox_config.AppConfig{Config: conf}
	srv := server.NewServer(ac, ul, nt)

	// 3. Start Server
	go func() {
		_ = srv.Start()
	}()
	defer srv.Stop()
	time.Sleep(500 * time.Millisecond)

	// 4. Simulate a client sending a notification
	client, err := factory.Create("tcp-hello", "127.0.0.1:10001", "127.0.0.1", "client", true)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Use the core handler to serialize a message
	handler := notifie.NewNotifHandler("ClientSimulator", conf)
	testMsg := &utils.NotifMessage{
		Message: "CRITICAL: Reactor Leak Detected!",
		Tags:    []string{"alert"},
	}
	serialized := handler.NotifNcapSerialize(testMsg)

	// Send raw message to server
	err = client.Send(serialized)
	assert.NoError(t, err)

	// 5. Verify the message reached the Notifie sender
	select {
	case received := <-ms.received:
		assert.Equal(t, "CRITICAL: Reactor Leak Detected!", received)
		fmt.Println("Integration Success: Message reached mock sender.")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout: Message never reached mock sender")
	}
}
