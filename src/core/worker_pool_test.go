package notifier

/*
ESSENTIAL PROCESS:
Validates the concurrent behavior and isolation of platform-specific worker pools.
Ensures that slow workers do not impact the throughput of fast workers.

DATA FLOW:
1. Setup multiple worker pools (Fast vs. Slow).
2. Burst messages across both pools.
3. Verify processed counts using atomic counters.
4. Validate that isolation holds under load.
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

type counterMockSender struct {
	count int32
}

func (m *counterMockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	atomic.AddInt32(&m.count, 1)
	// Simulate some work
	time.Sleep(10 * time.Millisecond)
	return nil
}
func (m *counterMockSender) GetTag() string      { return "counter" }
func (m *counterMockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestWorkerPoolDispatch(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "PoolTest")

	sender := &counterMockSender{}

	// Register manually to control parameters
	queue := make(chan *utils.NotifMessage, 100)
	n.senderQueues["counter"] = queue
	n.TagToSenderMap["counter"] = sender

	// Start 5 workers
	for i := 0; i < 5; i++ {
		go n.startSenderWorker("counter", sender, queue)
	}

	// Burst of 50 messages
	msg := &utils.NotifMessage{Message: "Log", Tags: []string{"counter"}}
	for i := 0; i < 50; i++ {
		_ = n.Notify(msg)
	}

	// Wait for workers to drain the queue
	time.Sleep(500 * time.Millisecond)

	assert.Equal(t, int32(50), atomic.LoadInt32(&sender.count), "All 50 messages should have been processed by the pool")
}

// -----------------------------------------------------------------------------

func TestWorkerPoolIsolation(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "IsolationTest")

	fastSender := &counterMockSender{}
	slowSender := &blockingMockSender{delay: 500 * time.Millisecond}

	// Setup Fast Pool
	fastQueue := make(chan *utils.NotifMessage, 100)
	n.senderQueues["fast"] = fastQueue
	n.TagToSenderMap["fast"] = fastSender
	go n.startSenderWorker("fast", fastSender, fastQueue)

	// Setup Slow/Blocking Pool
	slowQueue := make(chan *utils.NotifMessage, 100)
	n.senderQueues["slow"] = slowQueue
	n.TagToSenderMap["slow"] = slowSender
	go n.startSenderWorker("slow", slowSender, slowQueue)

	// Send messages to both
	msgBoth := &utils.NotifMessage{Message: "Sync", Tags: []string{"fast", "slow"}}
	for i := 0; i < 10; i++ {
		_ = n.Notify(msgBoth)
	}

	// Wait a moment for processing to complete
	time.Sleep(300 * time.Millisecond)

	// Fast pool should be done, slow pool should still be working
	assert.Equal(t, int32(10), atomic.LoadInt32(&fastSender.count), "Fast sender should have finished all 10")
	assert.Equal(t, int32(1), atomic.LoadInt32(&slowSender.calledCount), "Slow sender should still be on the first message")
}
