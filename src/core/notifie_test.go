package notifie

import (
	"testing"
	"time"

	"github.com/Bastien-Antigravity/universal-logger/src/config"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
	"github.com/stretchr/testify/assert"
)

type mockSender struct {
	lastMsg string
	lastTo  string
	lastSub string
	called  bool
}

func (m *mockSender) SendMessage(msg, to, subject string) error {
	m.lastMsg = msg
	m.lastTo = to
	m.lastSub = subject
	m.called = true
	return nil
}

func TestNotifieMessageFlow(t *testing.T) {
	// Initialize config for the test
	conf := config.NewDistributedConfig("test")
	
	// Create Notifie instance
	n := NewNotifie(conf, "TestParent")

	// Create and register mock sender
	mock := &mockSender{}
	n.TagToSenderMap["testTag"] = mock

	// Create a test message
	msg := &utils.NotifMessage{
		Message: "Hello Test Notification",
		Tags:    []string{"testTag"},
	}

	// Send message
	n.Send(msg)

	// Since processing is async (goroutines), wait a bit
	time.Sleep(150 * time.Millisecond)

	// Verify sender was called with correct data
	assert.True(t, mock.called, "Mock sender should have been called")
	assert.Equal(t, "Hello Test Notification", mock.lastMsg)
	assert.Equal(t, "TestParent", mock.lastSub) // In processMessage, parent name is used as subject
}

func TestRawMessageConsumption(t *testing.T) {
	conf := config.NewDistributedConfig("test")
	n := NewNotifie(conf, "RawTest")

	mock := &mockSender{}
	n.TagToSenderMap["rawTag"] = mock

	// Create a message and serialize it
	originalMsg := &utils.NotifMessage{
		Message: "Raw Secret Message",
		Tags:    []string{"rawTag"},
	}
	
	handler := NewNotifHandler("test", conf)
	rawData := handler.NotifNcapSerialize(originalMsg)

	// Send raw data
	n.SendRaw(rawData)

	// Wait for processing
	time.Sleep(150 * time.Millisecond)

	assert.True(t, mock.called, "Mock sender should have been called via raw consumption")
	assert.Equal(t, "Raw Secret Message", mock.lastMsg)
}
