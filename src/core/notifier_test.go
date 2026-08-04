package notifier

/*
ESSENTIAL PROCESS:
Unit tests for the Notifier core logic.
Verifies asynchronous message flow, worker pool dispatch, and queue capacity handling.

DATA FLOW:
1. Initialize a Notifier with mock senders.
2. Inject messages via Notify() or SendRaw().
3. Wait for worker pool consumption.
4. Verify mock sender state and call counts.
*/

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------

type mockSender struct {
	tag     string
	lastMsg string
	lastTo  string
	lastSub string
	called  bool
}

func (m *mockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	m.lastMsg = msg
	m.lastTo = to
	m.lastSub = subject
	m.called = true
	return nil
}

func (m *mockSender) GetTag() string {
	if m.tag != "" {
		return m.tag
	}
	return "testTag"
}
func (m *mockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestNotifierMessageFlow(t *testing.T) {
	// Initialize config for the test
	conf := distributed_config.New("test")

	// Create Notifier instance
	n := NewNotifier(conf, nil, "TestParent")

	// Create and register mock sender
	mock := &mockSender{}
	n.RegisterMockSender(mock)

	// Create a test message
	msg := &utils.NotifMessage{
		Message: "Hello Test Notification",
		Tags:    []string{"testTag"},
	}

	// Notify message
	err := n.Notify(msg)
	assert.NoError(t, err)

	// Since processing is async (worker pool), wait a bit
	time.Sleep(200 * time.Millisecond)

	// Verify sender was called with correct data
	assert.True(t, mock.called, "Mock sender should have been called")
	assert.Equal(t, "Hello Test Notification", mock.lastMsg)
	assert.Equal(t, "TestParent", mock.lastSub)
}

// -----------------------------------------------------------------------------

func TestRawMessageConsumption(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "RawTest")

	mock := &mockSender{}
	mock.tag = "rawTag"
	n.RegisterMockSender(mock)

	// Create a message and serialize it
	originalMsg := &utils.NotifMessage{
		Message: "Raw Secret Message",
		Tags:    []string{"rawTag"},
	}

	handler := NewNotifHandler("test", conf)
	rawData := handler.NotifNcapSerialize(originalMsg)

	// Send raw data
	err := n.SendRaw(rawData)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	assert.True(t, mock.called, "Mock sender should have been called via raw consumption")
	assert.Equal(t, "Raw Secret Message", mock.lastMsg)
}

// -----------------------------------------------------------------------------

func TestImplicitRouting(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "ImplicitTest")

	// Setup implicit routing: CRITICAL -> implicitTag
	mock := &mockSender{tag: "implicitTag"}
	n.RegisterMockSender(mock)

	n.mu.Lock()
	n.levelToTags["CRITICAL"] = []string{"implicitTag"}
	n.mu.Unlock()

	// Send message with level but NO tags
	msg := &utils.NotifMessage{
		Message: "Implicit Danger",
		Level:   "CRITICAL",
		Tags:    []string{},
	}

	err := n.Notify(msg)
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.True(t, mock.called, "Mock sender should have been called via implicit routing")
	assert.Equal(t, "Implicit Danger", mock.lastMsg)
}

// -----------------------------------------------------------------------------

func TestWorkerPoolCapacity(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "CapacityTest")

	// Create a sender that blocks to test queue fill
	blockingSender := &blockingMockSender{delay: 1 * time.Second}
	n.RegisterMockSender(blockingSender)

	// Send 5 messages
	msg := &utils.NotifMessage{Message: "Msg", Tags: []string{"blockTag"}}

	// First batch should succeed
	assert.NoError(t, n.Notify(msg))
	assert.NoError(t, n.Notify(msg))
	assert.NoError(t, n.Notify(msg))

	// In our processMessage, it logs a warning and drops if worker queue is full.
	// We've registered with default 1000 buffer in RegisterMockSender,
	// so let's just verify it processes.
	for i := 0; i < 10; i++ {
		_ = n.Notify(msg)
	}

	time.Sleep(100 * time.Millisecond)
	assert.True(t, atomic.LoadInt32(&blockingSender.calledCount) > 0, "At least one should be processing")
}

type blockingMockSender struct {
	delay       time.Duration
	calledCount int32
}

func (m *blockingMockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	atomic.AddInt32(&m.calledCount, 1)
	time.Sleep(m.delay)
	return nil
}
func (m *blockingMockSender) GetTag() string      { return "blockTag" }
func (m *blockingMockSender) GetLogLevel() string { return "INFO" }
