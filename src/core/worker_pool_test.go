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
func (m *counterMockSender) GetTag() string      { return "fast" } // Changed to match test usage
func (m *counterMockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestWorkerPoolDispatch(t *testing.T) {
	conf := distributed_config.New("test")
	n := NewNotifier(conf, nil, "PoolTest")

	sender := &counterMockSender{}
	n.RegisterMockSender(sender)

	// Burst of 50 messages
	msg := &utils.NotifMessage{Message: "Log", Tags: []string{"fast"}}
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

	// Register with specific tags if needed, but here we just register them
	// Note: counterMockSender.GetTag returns "fast"
	// blockingMockSender.GetTag returns "blockTag" (from notifier_test.go)
	n.RegisterMockSender(fastSender)
	n.RegisterMockSender(slowSender)

	// Send messages to both
	msgBoth := &utils.NotifMessage{Message: "Sync", Tags: []string{"fast", "blockTag"}}
	for i := 0; i < 10; i++ {
		_ = n.Notify(msgBoth)
	}

	// Wait a moment for processing to complete
	time.Sleep(300 * time.Millisecond)

	// Fast pool should be done, slow pool should still be working
	assert.Equal(t, int32(10), atomic.LoadInt32(&fastSender.count), "Fast sender should have finished all 10")
	// Since there are 5 workers by default, up to 5 could have started processing
	assert.True(t, atomic.LoadInt32(&slowSender.calledCount) <= 5, "Slow sender should not have finished everything")
}
