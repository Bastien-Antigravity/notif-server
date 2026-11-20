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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	toolbox_bootstrap "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/bootstrap"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------

var (
	testAppConfig *toolbox_config.AppConfig
	testLogger    log_interfaces.Logger
)

func init() {
	_ = os.Setenv("LOGGER_PROFILE", "minimal")
	var err error
	testAppConfig, testLogger, err = toolbox_bootstrap.BootstrapServiceSafe("notif-server-test")
	if err != nil {
		panic(err)
	}
}

// -----------------------------------------------------------------------------

type mockSender struct {
	mu      sync.RWMutex
	tag     string
	lastMsg string
	lastTo  string
	lastSub string
	called  bool
}

func (m *mockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastMsg = msg
	m.lastTo = to
	m.lastSub = subject
	m.called = true
	return nil
}

func (m *mockSender) isCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.called
}

func (m *mockSender) setCalled(val bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = val
}

func (m *mockSender) getLastMsg() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastMsg
}

func (m *mockSender) getLastSub() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSub
}

func (m *mockSender) GetTag() string {
	if m.tag == "" {
		return "mock"
	}
	return m.tag
}
func (m *mockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestNotifierMessageFlow(t *testing.T) {
	// Initialize config for the test
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)

	// Create Notifier instance
	n := NewNotifier(ac, testLogger, "TestParent")

	// Create and register mock sender
	mock := &mockSender{tag: "testTag"}
	n.RegisterSender(mock)

	// Create a test message
	msg := &utils.NotifMessage{
		Message: "Hello Test Notification",
		Tags:    []string{"testTag"},
	}

	// Notify message
	err = n.Notify(msg)
	assert.NoError(t, err)

	// Since processing is async (worker pool), wait a bit
	time.Sleep(200 * time.Millisecond)

	// Verify sender was called with correct data
	assert.True(t, mock.isCalled(), "Mock sender should have been called")
	assert.Equal(t, "Hello Test Notification", mock.getLastMsg())
	assert.Equal(t, "TestParent", mock.getLastSub())
}

// -----------------------------------------------------------------------------

func TestRawMessageConsumption(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "RawTest")

	mock := &mockSender{}
	mock.tag = "rawTag"
	n.RegisterSender(mock)

	// Create a message and serialize it
	originalMsg := &utils.NotifMessage{
		Message: "Raw Secret Message",
		Tags:    []string{"rawTag"},
	}

	handler := NewNotifHandler("test")
	rawData := handler.NotifNcapSerialize(originalMsg)

	// Send raw data
	err = n.SendRaw(rawData)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	assert.True(t, mock.isCalled(), "Mock sender should have been called via raw consumption")
	assert.Equal(t, "Raw Secret Message", mock.getLastMsg())
}

// -----------------------------------------------------------------------------

func TestImplicitRouting(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "ImplicitTest")

	// Setup implicit routing: CRITICAL -> implicitTag
	mock := &mockSender{tag: "implicitTag"}
	n.RegisterSender(mock)

	n.mu.Lock()
	n.levelToTags["CRITICAL"] = []string{"implicitTag"}
	n.mu.Unlock()

	// Send message with level but NO tags
	msg := &utils.NotifMessage{
		Message: "Implicit Danger",
		Level:   "CRITICAL",
		Tags:    []string{},
	}

	err = n.Notify(msg)
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.True(t, mock.isCalled(), "Mock sender should have been called via implicit routing")
	assert.Equal(t, "Implicit Danger", mock.getLastMsg())
}

// -----------------------------------------------------------------------------

func TestWorkerPoolCapacity(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "CapacityTest")

	// Create a sender that blocks to test queue fill
	blockingSender := &blockingMockSender{delay: 1 * time.Second}
	n.RegisterSender(blockingSender)

	// Send 5 messages
	msg := &utils.NotifMessage{Message: "Msg", Tags: []string{"blockTag"}}

	// First batch should succeed
	assert.NoError(t, n.Notify(msg))
	assert.NoError(t, n.Notify(msg))
	assert.NoError(t, n.Notify(msg))

	// In our processMessage, it logs a warning and drops if worker queue is full.
	// We've registered with default 1000 buffer in RegisterSender,
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

// -----------------------------------------------------------------------------

type failingMockSender struct {
	tag string
	err error
}

func (m *failingMockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	return m.err
}
func (m *failingMockSender) GetTag() string      { return m.tag }
func (m *failingMockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestFallbackToTelegramOnSenderFailure(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "FallbackTest")
	defer n.Stop()

	// 1. Register failing discord sender
	discordMock := &failingMockSender{tag: "discord", err: fmt.Errorf("discord webhook unreachable")}
	n.RegisterSender(discordMock)

	// 2. Register succeeding telegram sender
	telegramMock := &mockSender{tag: "telegram"}
	n.RegisterSender(telegramMock)

	// 3. Send message explicitly tagged to discord
	msg := &utils.NotifMessage{
		Message: "Important trading signal",
		Tags:    []string{"discord"},
	}
	err = n.Notify(msg)
	require.NoError(t, err)

	// Wait for async dispatch and fallback
	time.Sleep(200 * time.Millisecond)

	// 4. Assert telegram sender received fallback alert
	assert.True(t, telegramMock.isCalled(), "Telegram should have received the fallback notification")
	assert.Contains(t, telegramMock.getLastMsg(), "[Notification Fallback]")
	assert.Contains(t, telegramMock.getLastMsg(), "discord")
	assert.Contains(t, telegramMock.getLastMsg(), "Important trading signal")
}

// -----------------------------------------------------------------------------

func TestDefaultRoutingFallbackBanner(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "DefaultBannerTest")

	teleMock := &mockSender{tag: "telegram"}
	n.RegisterSender(teleMock)

	n.mu.Lock()
	n.DefaultSender = teleMock
	n.DefaultTag = "telegram"
	n.mu.Unlock()

	// 1. Untagged message with empty level
	msg1 := &utils.NotifMessage{
		Message: "Untagged alert message",
		Tags:    []string{},
		Level:   "",
	}
	err = n.Notify(msg1)
	require.NoError(t, err)
	time.Sleep(150 * time.Millisecond)

	assert.True(t, teleMock.isCalled())
	assert.Contains(t, teleMock.getLastMsg(), "⚠️ [Default Fallback - Untagged Alert]")
	assert.Contains(t, teleMock.getLastMsg(), "Untagged alert message")

	// 2. Message with unmapped tag
	teleMock.setCalled(false)
	msg2 := &utils.NotifMessage{
		Message: "Unmapped tag message",
		Tags:    []string{"non_existent_provider"},
		Level:   "",
	}
	err = n.Notify(msg2)
	require.NoError(t, err)
	time.Sleep(150 * time.Millisecond)

	assert.True(t, teleMock.isCalled())
	assert.Contains(t, teleMock.getLastMsg(), "⚠️ [Default Fallback - Unmapped Tags: [non_existent_provider]]")
	assert.Contains(t, teleMock.getLastMsg(), "Unmapped tag message")

	// 3. Message with unmapped level
	teleMock.setCalled(false)
	msg3 := &utils.NotifMessage{
		Message: "Unmapped level message",
		Tags:    []string{},
		Level:   "CUSTOM_UNKNOWN",
	}
	err = n.Notify(msg3)
	require.NoError(t, err)
	time.Sleep(150 * time.Millisecond)

	assert.True(t, teleMock.isCalled())
	assert.Contains(t, teleMock.getLastMsg(), "⚠️ [Default Fallback - Unmapped Level: CUSTOM_UNKNOWN]")
	assert.Contains(t, teleMock.getLastMsg(), "Unmapped level message")

	// 4. Properly mapped message (no fallback banner)
	teleMock.setCalled(false)
	msg4 := &utils.NotifMessage{
		Message: "Clean mapped alert",
		Tags:    []string{"telegram"},
		Level:   "CRITICAL",
	}
	err = n.Notify(msg4)
	require.NoError(t, err)
	time.Sleep(150 * time.Millisecond)

	assert.True(t, teleMock.isCalled())
	assert.NotContains(t, teleMock.getLastMsg(), "[Default Fallback")
	assert.Equal(t, "Clean mapped alert", teleMock.getLastMsg())
}

// -----------------------------------------------------------------------------

type collectingMockSender struct {
	mu       sync.Mutex
	tag      string
	messages []string
	delay    time.Duration
}

func (m *collectingMockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()
	return nil
}

func (m *collectingMockSender) GetTag() string      { return m.tag }
func (m *collectingMockSender) GetLogLevel() string { return "INFO" }

func (m *collectingMockSender) getMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]string, len(m.messages))
	copy(copied, m.messages)
	return copied
}

// -----------------------------------------------------------------------------

func TestGracefulSenderReplacementDrainsPending(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "DrainTest")
	defer n.Stop()

	// 1. Register Sender 1 with a small processing delay
	sender1 := &collectingMockSender{tag: "drainTag", delay: 20 * time.Millisecond}
	n.RegisterSender(sender1)

	// 2. Queue 5 messages to Sender 1
	for i := 1; i <= 5; i++ {
		err := n.Notify(&utils.NotifMessage{
			Message: fmt.Sprintf("Pending-%d", i),
			Tags:    []string{"drainTag"},
		})
		require.NoError(t, err)
	}

	// Small pause to allow messages to reach sender1's queue
	time.Sleep(10 * time.Millisecond)

	// 3. Immediately replace with Sender 2 while Sender 1 is still processing
	sender2 := &collectingMockSender{tag: "drainTag"}
	n.RegisterSender(sender2)

	// 4. Queue 2 new messages to the replacement sender
	for i := 1; i <= 2; i++ {
		err := n.Notify(&utils.NotifMessage{
			Message: fmt.Sprintf("New-%d", i),
			Tags:    []string{"drainTag"},
		})
		require.NoError(t, err)
	}

	// 5. Wait for both senders to complete
	assert.Eventually(t, func() bool {
		return len(sender1.getMessages()) == 5 && len(sender2.getMessages()) == 2
	}, 3*time.Second, 20*time.Millisecond, "Sender 1 must drain all 5 pending messages, and Sender 2 must receive its 2 new messages")

	// Verify exact content
	assert.Equal(t, []string{"Pending-1", "Pending-2", "Pending-3", "Pending-4", "Pending-5"}, sender1.getMessages())
	assert.Equal(t, []string{"New-1", "New-2"}, sender2.getMessages())
}

// -----------------------------------------------------------------------------

func TestRegisterSenderDeduplicationNoWorkerLeak(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "DeduplicationTest")
	defer n.Stop()

	// 1. Repeatedly register 10 senders with the same tag
	const iterations = 10
	var finalSender *collectingMockSender
	for i := 0; i < iterations; i++ {
		s := &collectingMockSender{tag: "dedupTag"}
		n.RegisterSender(s)
		finalSender = s
	}

	// 2. Send 5 messages
	for i := 1; i <= 5; i++ {
		err := n.Notify(&utils.NotifMessage{
			Message: fmt.Sprintf("DedupMsg-%d", i),
			Tags:    []string{"dedupTag"},
		})
		require.NoError(t, err)
	}

	// 3. Wait for final sender to receive all 5 messages
	assert.Eventually(t, func() bool {
		return len(finalSender.getMessages()) == 5
	}, 2*time.Second, 20*time.Millisecond, "Final sender should receive exactly 5 messages")

	// Give a moment for any leaked workers to finish/react
	time.Sleep(100 * time.Millisecond)

	// Verify exactly 5 messages received without duplicate deliveries
	assert.Equal(t, 5, len(finalSender.getMessages()))

	// Ensure internal senderQueues only maintains one queue for dedupTag
	n.mu.RLock()
	queueCount := len(n.senderQueues)
	_, hasQueue := n.senderQueues["dedupTag"]
	n.mu.RUnlock()

	assert.True(t, hasQueue)
	assert.True(t, queueCount <= 2, "Should only have dedupTag (and optionally default telegram) in senderQueues")
}

// -----------------------------------------------------------------------------

func TestNotifierGracefulShutdownIdempotentAndDrains(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "ShutdownDrainTest")

	sender := &collectingMockSender{tag: "drainTag"}
	n.RegisterSender(sender)

	// Send 10 messages into notifier
	for i := 1; i <= 10; i++ {
		err := n.Notify(&utils.NotifMessage{
			Message: fmt.Sprintf("ShutdownMsg-%d", i),
			Tags:    []string{"drainTag"},
		})
		require.NoError(t, err)
	}

	// Stop notifier (should drain NotifChan and senderQueue before returning)
	n.Stop()

	// Calling Stop() a second time must be idempotent and not panic
	assert.NotPanics(t, func() {
		n.Stop()
	})

	// Attempting to send new notifications after Stop() must be rejected
	err = n.Notify(&utils.NotifMessage{
		Message: "PostShutdownMsg",
		Tags:    []string{"drainTag"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutting down")

	// All 10 pre-shutdown messages must have been delivered cleanly
	messages := sender.getMessages()
	assert.Equal(t, 10, len(messages), "All 10 queued messages must be delivered during graceful shutdown")
}

// -----------------------------------------------------------------------------

type toggleMockSender struct {
	mu      sync.RWMutex
	tag     string
	failErr error
}

func (m *toggleMockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	m.mu.RLock()
	err := m.failErr
	m.mu.RUnlock()
	return err
}

func (m *toggleMockSender) setError(err error) {
	m.mu.Lock()
	m.failErr = err
	m.mu.Unlock()
}

func (m *toggleMockSender) GetTag() string      { return m.tag }
func (m *toggleMockSender) GetLogLevel() string { return "INFO" }

func findNotifierStatus(statuses []NotifierStatus, tag string) (NotifierStatus, bool) {
	for _, s := range statuses {
		if s.Name == tag {
			return s, true
		}
	}
	return NotifierStatus{}, false
}

// -----------------------------------------------------------------------------

func TestNotifierDynamicHealthMetricsTransitions(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "HealthMetricsTest")
	defer n.Stop()

	// Mount a succeeding default telegram mock so fallback dispatches don't touch real HTTP
	n.RegisterSender(&mockSender{tag: "telegram"})

	sender := &toggleMockSender{tag: "healthTag"}
	n.RegisterSender(sender)

	// 1. Initialized state before any deliveries
	st, ok := findNotifierStatus(n.GetActiveNotifiers(), "healthTag")
	require.True(t, ok)
	assert.Equal(t, "Initialized", st.State)
	assert.True(t, st.Healthy)
	assert.Equal(t, uint64(0), st.DeliveredCount)

	// 2. Operational state after successful delivery
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "ok-1", Tags: []string{"healthTag"}}))
	assert.Eventually(t, func() bool {
		st, ok = findNotifierStatus(n.GetActiveNotifiers(), "healthTag")
		return ok && st.DeliveredCount == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "Operational", st.State)
	assert.True(t, st.Healthy)
	assert.False(t, st.LastSuccess.IsZero())

	// 3. Degraded state after 1 failure
	sender.setError(fmt.Errorf("transient 503"))
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "fail-1", Tags: []string{"healthTag"}}))
	assert.Eventually(t, func() bool {
		st, ok = findNotifierStatus(n.GetActiveNotifiers(), "healthTag")
		return ok && st.FailedCount == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "Degraded", st.State)
	assert.False(t, st.Healthy)
	assert.Equal(t, int64(1), st.ConsecutiveFailures)
	assert.Contains(t, st.LastError, "transient 503")

	// 4. Unavailable state after 3 consecutive failures
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "fail-2", Tags: []string{"healthTag"}}))
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "fail-3", Tags: []string{"healthTag"}}))
	assert.Eventually(t, func() bool {
		st, ok = findNotifierStatus(n.GetActiveNotifiers(), "healthTag")
		return ok && st.ConsecutiveFailures == 3
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "Unavailable", st.State)
	assert.False(t, st.Healthy)

	// Check Controller.GetStatus reflects degraded/unavailable provider
	ctrl := NewController(n)
	details, err := ctrl.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "Unavailable", details["provider_healthTag_state"])
	assert.Equal(t, "1", details["degraded_notifiers"])
	assert.Contains(t, details["provider_healthTag_last_error"], "transient 503")

	// 5. Recovery back to Operational on next success
	sender.setError(nil)
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "recover-1", Tags: []string{"healthTag"}}))
	assert.Eventually(t, func() bool {
		st, ok = findNotifierStatus(n.GetActiveNotifiers(), "healthTag")
		return ok && st.DeliveredCount == 2
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "Operational", st.State)
	assert.True(t, st.Healthy)
	assert.Equal(t, int64(0), st.ConsecutiveFailures)
	assert.Empty(t, st.LastError)
}

// -----------------------------------------------------------------------------

type channelBlockingMockSender struct {
	tag     string
	unblock chan struct{}
}

func (m *channelBlockingMockSender) SendMessage(ctx context.Context, msg, to, subject string) error {
	select {
	case <-m.unblock:
	case <-ctx.Done():
	}
	return nil
}

func (m *channelBlockingMockSender) GetTag() string      { return m.tag }
func (m *channelBlockingMockSender) GetLogLevel() string { return "INFO" }

// -----------------------------------------------------------------------------

func TestNotifierDropCountersUnderBackpressure(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "DropCounterTest")

	unblock := make(chan struct{})
	defer func() {
		close(unblock)
		n.Stop()
	}()

	blocker := &channelBlockingMockSender{tag: "slowTag", unblock: unblock}
	n.RegisterSender(blocker)

	// Flood more than 100 messages (worker queue capacity is 100) while worker is blocked
	for i := 0; i < 140; i++ {
		_ = n.Notify(&utils.NotifMessage{
			Message: fmt.Sprintf("Spike-%d", i),
			Tags:    []string{"slowTag"},
		})
	}

	// Verify backpressure is tracked via DroppedCount (and/or dropped_ingestion) and Degraded state
	assert.Eventually(t, func() bool {
		st, ok := findNotifierStatus(n.GetActiveNotifiers(), "slowTag")
		if !ok {
			return false
		}
		return (st.DroppedCount+n.GetDroppedIngestionCount()) > 0 && st.State == "Degraded" && !st.Healthy
	}, 2*time.Second, 10*time.Millisecond, "Saturated worker queue must record dropped alerts and report Degraded state")

	ctrl := NewController(n)
	details, err := ctrl.GetStatus()
	require.NoError(t, err)
	assert.NotEqual(t, "0", details["total_dropped"])
}

// -----------------------------------------------------------------------------

func TestLiveConfigHotReloadOnTheFlyAndMultipleSameTypeSenders(t *testing.T) {
	var mu sync.Mutex
	var hitsV1, hitsV2, hitsSecondary []string

	newMockHook := func(dest *[]string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			*dest = append(*dest, string(body))
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}))
	}

	serverV1 := newMockHook(&hitsV1)
	defer serverV1.Close()
	serverV2 := newMockHook(&hitsV2)
	defer serverV2.Close()
	serverSecondary := newMockHook(&hitsSecondary)
	defer serverSecondary.Close()

	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "LiveReloadTest")
	defer n.Stop()

	ctrl := NewController(n)

	// 1. Mount TWO senders of the SAME platform type (DISCORD) with different section names
	// (Also test custom TAG differing from sectionName on discord_secondary!)
	n.Reload(map[string]map[string]string{
		"discord_primary": {
			"TYPE":     "DISCORD",
			"URL":      serverV1.URL,
			"LOGLEVEL": "INFO",
		},
		"discord_secondary": {
			"TYPE":     "DISCORD",
			"TAG":      "custom_secondary_label",
			"URL":      serverSecondary.URL,
			"LOGLEVEL": "INFO",
		},
	})

	// Send to both simultaneously
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "hello-v1", Tags: []string{"discord_primary"}}))
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "hello-secondary", Tags: []string{"discord_secondary"}}))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(hitsV1) == 1 && len(hitsSecondary) == 1
	}, 3*time.Second, 20*time.Millisecond, "Both same-type Discord senders must deliver independently")

	// 2. Change discord_primary's URL ON THE FLY via Controller.SetAlertingConfig (no restart!)
	require.NoError(t, ctrl.SetAlertingConfig("discord_primary", "URL", serverV2.URL))

	// Send new messages to both
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "hello-v2-live", Tags: []string{"discord_primary"}}))
	require.NoError(t, n.Notify(&utils.NotifMessage{Message: "hello-secondary-2", Tags: []string{"discord_secondary"}}))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(hitsV2) == 1 && len(hitsSecondary) == 2
	}, 3*time.Second, 20*time.Millisecond, "Updated sender must immediately route to new URL while sibling sender continues uninterrupted")

	mu.Lock()
	assert.Len(t, hitsV1, 1, "Old endpoint V1 must not receive messages sent after live reload")
	assert.Contains(t, hitsV2[0], "hello-v2-live")
	assert.Contains(t, hitsSecondary[1], "hello-secondary-2")
	mu.Unlock()
}

// -----------------------------------------------------------------------------

func TestReloadAndRegisterAfterStopAreSafe(t *testing.T) {
	ac, err := toolbox_config.LoadConfig("standalone", nil)
	require.NoError(t, err)
	n := NewNotifier(ac, testLogger, "PostStopSafetyTest")

	n.Stop()

	assert.NotPanics(t, func() {
		n.Reload(map[string]map[string]string{
			"discord_late": {"TYPE": "DISCORD", "URL": "https://example.com/hook"},
		})
		n.RegisterSender(&mockSender{tag: "lateTag"})
		n.InitDefaultProviders()
	})
}
