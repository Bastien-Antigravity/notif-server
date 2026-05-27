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

func (m *mockSender) GetTag() string      { return "testTag" }
func (m *mockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestNotifierMessageFlow(t *testing.T) {
	// Initialize config for the test
	conf := distributed_config.New("test")

	// Create Notifier instance
	n := NewNotifier(conf, nil, "TestParent")

	// Create and register mock sender
	mock := &mockSender{}

	// Explicitly register in the worker pool system
	// This simulates what LoadNotifSender does
	queue := make(chan *utils.NotifMessage, 10)
	n.senderQueues["testTag"] = queue
	n.TagToSenderMap["testTag"] = mock
	go n.startSenderWorker("testTag", mock, queue)

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

	// Register worker
	queue := make(chan *utils.NotifMessage, 10)
	n.senderQueues["rawTag"] = queue
	n.TagToSenderMap["rawTag"] = mock
	go n.startSenderWorker("rawTag", mock, queue)

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

func TestWorkerPoolCapacity(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "CapacityTest")

	// Create a sender that blocks to test queue fill
	blockingSender := &blockingMockSender{delay: 1 * time.Second}

	// Register with tiny queue
	queue := make(chan *utils.NotifMessage, 2)
	n.senderQueues["blockTag"] = queue
	n.TagToSenderMap["blockTag"] = blockingSender
	// Only 1 worker to ensure serial blocking
	go n.startSenderWorker("blockTag", blockingSender, queue)

	// Send 5 messages
	msg := &utils.NotifMessage{Message: "Msg", Tags: []string{"blockTag"}}

	// First 3 should succeed (1 in worker, 2 in queue)
	assert.NoError(t, n.Notify(msg))
	assert.NoError(t, n.Notify(msg))
	assert.NoError(t, n.Notify(msg))

	// 4th should be dropped or return error depending on implementation
	// In our processMessage, it logs a warning and drops.
	// But Notify itself will succeed unless the main NotifChan is full.
	// Let's verify no panic occurs.
	for i := 0; i < 10; i++ {
		_ = n.Notify(msg)
	}

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&blockingSender.calledCount), "Only one should be currently processing")
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
