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

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, nil, "PoolTest")

	sender := &counterMockSender{}
	n.RegisterSender(sender)

	// Burst of 50 messages
	msg := &utils.NotifMessage{Message: "Log", Tags: []string{"fast"}}
	for i := 0; i < 50; i++ {
		_ = n.Notify(msg)
	}

	// Wait for workers to drain the queue
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&sender.count) == 50
	}, 3*time.Second, 20*time.Millisecond, "All 50 messages should have been processed by the pool")
}

// -----------------------------------------------------------------------------

func TestWorkerPoolIsolation(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, nil, "IsolationTest")

	fastSender := &counterMockSender{}
	slowSender := &blockingMockSender{delay: 500 * time.Millisecond}

	// Register with specific tags if needed, but here we just register them
	// Note: counterMockSender.GetTag returns "fast"
	// blockingMockSender.GetTag returns "blockTag" (from notifier_test.go)
	n.RegisterSender(fastSender)
	n.RegisterSender(slowSender)

	// Send messages to both
	msgBoth := &utils.NotifMessage{Message: "Sync", Tags: []string{"fast", "blockTag"}}
	for i := 0; i < 10; i++ {
		_ = n.Notify(msgBoth)
	}

	// Fast pool should be done, slow pool should still be working
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&fastSender.count) == 10
	}, 2*time.Second, 20*time.Millisecond, "Fast sender should have finished all 10")
	assert.True(t, atomic.LoadInt32(&slowSender.calledCount) <= 5, "Slow sender should not have finished everything")
}
