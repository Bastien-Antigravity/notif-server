package notifier

/*
ESSENTIAL PROCESS:
Manages the core notification dispatch logic, worker queues, and asynchronous delivery.
Ensures that slow external APIs do not block the ingestion layer.

DATA FLOW:
1. Ingestion layers (TCP/gRPC/universal-logger/controller) push messages to NotifChan.
2. Router resolves target platforms using explicit tags and implicit log-level mapping.
3. Messages are dispatched into platform-specific buffered worker queues.
4. Dedicated workers deliver messages to external sinks with a 30s timeout.

KEY PARAMETERS:
- NotifChan: Primary ingestion channel for structured notification messages.
- senderQueues: Map of platform tags to their respective worker queues.
- levelToTags: Mapping of log levels (e.g. CRITICAL, ERROR) to target provider tags.
*/

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Bastien-Antigravity/notif-server/src/interfaces"
	"github.com/Bastien-Antigravity/notif-server/src/notifiers"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// senderMetrics tracks delivery health and backpressure telemetry for a single provider.
type senderMetrics struct {
	mu                  sync.RWMutex
	delivered           uint64
	failed              uint64
	dropped             uint64
	consecutiveFailures int64
	lastSuccess         time.Time
	lastError           string
}

func (m *senderMetrics) recordSuccess() {
	m.mu.Lock()
	m.delivered++
	m.consecutiveFailures = 0
	m.lastSuccess = time.Now()
	m.lastError = ""
	m.mu.Unlock()
}

func (m *senderMetrics) recordFailure(err error) {
	m.mu.Lock()
	m.failed++
	m.consecutiveFailures++
	if err != nil {
		m.lastError = err.Error()
	}
	m.mu.Unlock()
}

func (m *senderMetrics) recordDrop() {
	m.mu.Lock()
	m.dropped++
	m.mu.Unlock()
}

func (m *senderMetrics) snapshot() (delivered, failed, dropped uint64, consecFail int64, lastSuccess time.Time, lastErr string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.delivered, m.failed, m.dropped, m.consecutiveFailures, m.lastSuccess, m.lastError
}

// -----------------------------------------------------------------------------

// Notifier is the concrete implementation of the notification service.
type Notifier struct {
	proto_msg.UnimplementedNotifServiceServer
	Name             string
	appConfig        *toolbox_config.AppConfig
	Logger           log_interfaces.Logger
	TagToSenderMap   map[string]interfaces.INotifSender
	DefaultSender    interfaces.INotifSender
	DefaultTag       string
	NotifChan        chan *utils.NotifMessage
	senderQueues     map[string]chan *utils.NotifMessage
	senderMetrics    map[string]*senderMetrics
	workerDone       map[string]chan struct{}
	currentConf      map[string]map[string]string
	levelToTags      map[string][]string // Implicit routing map: Level -> [Tag1, Tag2]
	mu               sync.RWMutex
	shutdown         chan struct{}
	stopOnce         sync.Once
	isStopping       bool
	droppedIngestion atomic.Uint64
	processWg        sync.WaitGroup
	workersWg        sync.WaitGroup
	OnUpdate         func()
}

// -----------------------------------------------------------------------------

// NewNotifier creates a new instance of the notification service.
func NewNotifier(appConfig *toolbox_config.AppConfig, logger log_interfaces.Logger, parentName string) *Notifier {
	curNotifier := &Notifier{
		Name:           parentName,
		appConfig:      appConfig,
		Logger:         logger,
		NotifChan:      make(chan *utils.NotifMessage, 100),
		TagToSenderMap: make(map[string]interfaces.INotifSender),
		senderQueues:   make(map[string]chan *utils.NotifMessage),
		senderMetrics:  make(map[string]*senderMetrics),
		workerDone:     make(map[string]chan struct{}),
		currentConf:    make(map[string]map[string]string),
		levelToTags:    make(map[string][]string),
		shutdown:       make(chan struct{}),
	}

	curNotifier.Logger.AddMetadata("component", "notifier")

	// 1. Initial Load from LiveConfig if present
	if liveConf := appConfig.Config.LiveConfig.Load(); liveConf != nil {
		curNotifier.Reload(*liveConf)
	}

	// 2. Initialize default providers from static capabilities (e.g. standalone execution)
	curNotifier.InitDefaultProviders()

	// 3. Register for Live Updates
	appConfig.Config.OnLiveConfUpdate(func(newConf map[string]map[string]string) {
		curNotifier.Logger.Info("Live configuration update received. Reloading senders...")
		curNotifier.Reload(newConf)
		curNotifier.InitDefaultProviders()
	})

	curNotifier.processWg.Add(1)
	go curNotifier.processMessage()

	return curNotifier
}

// -----------------------------------------------------------------------------

// InitDefaultProviders initializes baseline providers (like telegram) from static capabilities
// if no dynamic providers have been registered for them.
func (notifier *Notifier) InitDefaultProviders() {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	if notifier.isStopping {
		return
	}

	if _, exists := notifier.TagToSenderMap["telegram"]; exists {
		return
	}

	var token, chatID, url string

	var notifCap struct {
		Token  string `json:"token"`
		ChatID string `json:"chat_id"`
		ChatId string `json:"chatid"`
		URL    string `json:"url"`
	}
	if err := notifier.appConfig.GetCapability("notif_server", &notifCap); err == nil {
		token = notifCap.Token
		if notifCap.ChatID != "" {
			chatID = notifCap.ChatID
		} else {
			chatID = notifCap.ChatId
		}
		url = notifCap.URL
	}

	if token == "" || chatID == "" {
		notifier.Logger.Error("Missing required Telegram credentials (token/chat_id) in capabilities.notif_server")
		return
	}

	if url == "" {
		url = "https://api.telegram.org"
	}
	conf := map[string]string{
		"TYPE":     "TELEGRAM",
		"TAG":      "telegram",
		"TOKEN":    token,
		"CHATID":   chatID,
		"URL":      url,
		"LOGLEVEL": "CRITICAL,ERROR,WARNING,INFO",
	}
	notifier.startSenderLocked("TELEGRAM", "telegram", conf, nil)

	// Designate Telegram as the authoritative main default provider and universal fallback
	if tele, ok := notifier.TagToSenderMap["telegram"]; ok {
		notifier.DefaultSender = tele
		notifier.DefaultTag = "telegram"
	}

	// Register default implicit routing levels for telegram
	for _, lvl := range []string{"CRITICAL", "ERROR", "WARNING"} {
		notifier.addLevelTagLocked(lvl, "telegram")
	}

	notifier.Logger.Info("Default main Telegram notification provider mounted from static capabilities (routing: CRITICAL,ERROR,WARNING)")
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) addLevelTagLocked(level, tag string) {
	for _, existing := range notifier.levelToTags[level] {
		if existing == tag {
			return
		}
	}
	notifier.levelToTags[level] = append(notifier.levelToTags[level], tag)
}

// -----------------------------------------------------------------------------

// Reload compares the new configuration with the current state and hot-swaps
// senders that have changed or were added/removed.
func (notifier *Notifier) Reload(newConf map[string]map[string]string) {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	if notifier.isStopping {
		return
	}

	supportedTypes := map[string]bool{
		"TELEGRAM": true,
		"DISCORD":  true,
		"MATRIX":   true,
		"GMAIL":    true,
	}

	activeTags := make(map[string]bool)
	newLevelToTags := make(map[string][]string)

	// 1. Identify and Start/Update Notifiers from Config
	for sectionName, sectionConf := range newConf {
		platType, ok := sectionConf["TYPE"]
		if !ok {
			platType, ok = sectionConf["type"]
		}
		if !ok {
			// Fallback: If no TYPE but name matches a platform, treat as that type
			if supportedTypes[strings.ToUpper(sectionName)] {
				platType = strings.ToUpper(sectionName)
			} else {
				continue
			}
		}

		platType = strings.ToUpper(platType)
		if !supportedTypes[platType] {
			continue
		}

		activeTags[sectionName] = true
		oldConf, existed := notifier.currentConf[sectionName]

		// Update or Start
		if !existed || !reflect.DeepEqual(sectionConf, oldConf) {
			var prevDone chan struct{}
			if existed {
				notifier.Logger.Info("Updating configuration for notifier: %s", sectionName)
				prevDone = notifier.stopSenderLocked(sectionName)
			} else {
				notifier.Logger.Info("Initializing new notifier: %s (Type: %s)", sectionName, platType)
			}
			notifier.startSenderLocked(platType, sectionName, sectionConf, prevDone)
		}

		// Update Level Routing Map
		if logLevels, ok := sectionConf["LOGLEVEL"]; ok {
			for _, level := range strings.Split(logLevels, ",") {
				level = strings.TrimSpace(strings.ToUpper(level))
				if level != "" {
					newLevelToTags[level] = append(newLevelToTags[level], sectionName)
				}
			}
		}
	}

	// 2. Stop Notifiers that were in currentConf but removed from newConf
	for existingTag := range notifier.currentConf {
		if !activeTags[existingTag] {
			notifier.Logger.Info("Removing notifier: %s", existingTag)
			notifier.stopSenderLocked(existingTag)
		}
	}

	// 3. Preserve default telegram provider if it's active and not overridden in newConf
	if _, hasTelegram := activeTags["telegram"]; !hasTelegram {
		if tele, isDefaultMounted := notifier.TagToSenderMap["telegram"]; isDefaultMounted {
			if notifier.DefaultSender == nil {
				notifier.DefaultSender = tele
				notifier.DefaultTag = "telegram"
			}
			for _, lvl := range []string{"CRITICAL", "ERROR", "WARNING"} {
				newLevelToTags[lvl] = append(newLevelToTags[lvl], "telegram")
			}
		}
	}

	notifier.currentConf = cloneConfig(newConf)
	notifier.levelToTags = newLevelToTags
	notifier.Logger.Info("Notifier routing map updated: %v", notifier.levelToTags)
	if notifier.OnUpdate != nil {
		go notifier.OnUpdate()
	}
}

// -----------------------------------------------------------------------------

// NotifierStatus contains health and telemetry info about an active provider.
type NotifierStatus struct {
	Name                string
	Type                string
	Healthy             bool
	State               string
	QueueDepth          int
	DeliveredCount      uint64
	FailedCount         uint64
	DroppedCount        uint64
	ConsecutiveFailures int64
	LastSuccess         time.Time
	LastError           string
}

// -----------------------------------------------------------------------------

// GetActiveNotifiers returns dynamic health and telemetry about all active notification senders.
func (notifier *Notifier) GetActiveNotifiers() []NotifierStatus {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	result := make([]NotifierStatus, 0, len(notifier.TagToSenderMap))
	for tag, sender := range notifier.TagToSenderMap {
		status := NotifierStatus{
			Name:  tag,
			Type:  reflect.TypeOf(sender).String(),
			State: "Initialized",
		}

		queueDepth := 0
		queueCap := 0
		if q, ok := notifier.senderQueues[tag]; ok {
			queueDepth = len(q)
			queueCap = cap(q)
		}
		status.QueueDepth = queueDepth

		if m, ok := notifier.senderMetrics[tag]; ok {
			delivered, failed, dropped, consecFail, lastSuccess, lastErr := m.snapshot()
			status.DeliveredCount = delivered
			status.FailedCount = failed
			status.DroppedCount = dropped
			status.ConsecutiveFailures = consecFail
			status.LastSuccess = lastSuccess
			status.LastError = lastErr
		}

		switch {
		case notifier.isStopping:
			status.State = "Stopped"
			status.Healthy = false
		case status.ConsecutiveFailures >= 3:
			status.State = "Unavailable"
			status.Healthy = false
		case status.ConsecutiveFailures > 0 || (queueCap > 0 && queueDepth*100/queueCap >= 80):
			status.State = "Degraded"
			status.Healthy = false
		case status.DeliveredCount > 0:
			status.State = "Operational"
			status.Healthy = true
		default:
			status.State = "Initialized"
			status.Healthy = true
		}

		result = append(result, status)
	}
	return result
}

// -----------------------------------------------------------------------------

// GetDroppedIngestionCount returns the total number of notifications dropped because NotifChan was full.
func (notifier *Notifier) GetDroppedIngestionCount() uint64 {
	return notifier.droppedIngestion.Load()
}

// -----------------------------------------------------------------------------

// GetIngestionQueueDepth returns the current number of pending notifications in NotifChan.
func (notifier *Notifier) GetIngestionQueueDepth() int {
	return len(notifier.NotifChan)
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) stopSenderLocked(tag string) chan struct{} {
	var prevDone chan struct{}
	if done, ok := notifier.workerDone[tag]; ok {
		prevDone = done
		delete(notifier.workerDone, tag)
	}
	if queue, ok := notifier.senderQueues[tag]; ok {
		close(queue)
		delete(notifier.senderQueues, tag)
	}
	delete(notifier.senderMetrics, tag)
	delete(notifier.TagToSenderMap, tag)

	if notifier.DefaultTag == tag {
		notifier.DefaultSender = nil
		notifier.DefaultTag = ""
	}

	for lvl, tags := range notifier.levelToTags {
		var filtered []string
		for _, t := range tags {
			if t != tag {
				filtered = append(filtered, t)
			}
		}
		notifier.levelToTags[lvl] = filtered
	}
	return prevDone
}

// -----------------------------------------------------------------------------

func isNilSender(sender interfaces.INotifSender) bool {
	if sender == nil {
		return true
	}
	val := reflect.ValueOf(sender)
	return val.Kind() == reflect.Ptr && val.IsNil()
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) startSenderLocked(platform, tag string, conf map[string]string, prevDone chan struct{}) {
	var sender interfaces.INotifSender
	var err error

	decrypt := notifier.appConfig.DecryptSecret

	switch platform {
	case "TELEGRAM":
		s, e := notifiers.NewTelegramSender(conf, tag, notifier.Logger, decrypt)
		if s != nil {
			sender = s
		}
		err = e
	case "DISCORD":
		s, e := notifiers.NewDiscordSender(conf, tag, notifier.Logger, decrypt)
		if s != nil {
			sender = s
		}
		err = e
	case "MATRIX":
		s, e := notifiers.NewMatrixSender(conf, tag, notifier.Logger, decrypt)
		if s != nil {
			sender = s
		}
		err = e
	case "GMAIL":
		s, e := notifiers.NewGmailSender(conf, tag, notifier.Logger, decrypt)
		if s != nil {
			sender = s
		}
		err = e
	default:
		notifier.Logger.Error("Unsupported notification platform '%s' for provider '%s'", platform, tag)
		return
	}

	if err != nil {
		notifier.Logger.Error("Failed to initialize %s (%s): %v", platform, tag, err)
		return
	}
	// If sender == nil and err == nil, provider is unconfigured (configuration is optional)
	if isNilSender(sender) {
		return
	}

	if prevDone == nil {
		if _, exists := notifier.senderQueues[tag]; exists {
			prevDone = notifier.stopSenderLocked(tag)
		}
	}

	notifier.TagToSenderMap[tag] = sender

	// Designate as default main provider if it is telegram or if no default provider is set yet
	if tag == "telegram" || notifier.DefaultSender == nil {
		notifier.DefaultSender = sender
		notifier.DefaultTag = tag
	}

	// Single worker per sender ensures sequential message ordering and avoids rate-limit violations
	notifier.startSenderWorkerLocked(tag, sender, prevDone)
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) startSenderWorkerLocked(tag string, sender interfaces.INotifSender, prevDone chan struct{}) {
	if isNilSender(sender) {
		return
	}
	if tag == "" {
		tag = sender.GetTag()
	}
	queue := make(chan *utils.NotifMessage, 100)
	notifier.senderQueues[tag] = queue

	metrics := &senderMetrics{}
	notifier.senderMetrics[tag] = metrics

	currentDone := make(chan struct{})
	notifier.workerDone[tag] = currentDone

	notifier.workersWg.Add(1)
	go func() {
		defer notifier.workersWg.Done()
		defer close(currentDone)

		// Wait for any prior worker for this tag to finish draining before starting
		if prevDone != nil {
			<-prevDone
		}

		notifier.startSenderWorker(tag, sender, queue, metrics)
	}()
}

// -----------------------------------------------------------------------------

// Notify sends a structured notification message.
func (notifier *Notifier) Notify(msg *utils.NotifMessage) error {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	if notifier.isStopping {
		return errors.New("notifier is shutting down; rejecting new notifications")
	}

	select {
	case notifier.NotifChan <- msg:
		return nil
	default:
		notifier.droppedIngestion.Add(1)
		return context.DeadlineExceeded // Buffer full
	}
}

// -----------------------------------------------------------------------------

// SendRaw deserializes a binary Cap'n Proto notification frame and queues it.
func (notifier *Notifier) SendRaw(data []byte) error {
	msg, err := DeserializeNotifMsg(data)
	if err != nil {
		notifier.Logger.Error("SendRaw: error deserializing message: %v", err)
		return err
	}
	return notifier.Notify(msg)
}

// -----------------------------------------------------------------------------

// SendNotification implements proto_msg.NotifServiceServer (gRPC)
func (notifier *Notifier) SendNotification(ctx context.Context, req *proto_msg.NotifRequest) (*proto_msg.NotifResponse, error) {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	if notifier.isStopping {
		return &proto_msg.NotifResponse{Success: false}, errors.New("notifier is shutting down; rejecting new notifications")
	}

	msg := &utils.NotifMessage{
		Message:    req.Message,
		Tags:       req.Tags,
		Attachment: req.Attachment,
		Level:      req.Level,
	}

	select {
	case notifier.NotifChan <- msg:
		return &proto_msg.NotifResponse{Success: true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		notifier.droppedIngestion.Add(1)
		return &proto_msg.NotifResponse{Success: false}, nil
	}
}

// -----------------------------------------------------------------------------

// LoadNotifSender is a wrapper around Reload for backward compatibility.
func (notifier *Notifier) LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string {
	notifier.Reload(notifiersConf)
	return nil
}

// -----------------------------------------------------------------------------

// Stop shuts down the notifier, drains all queues, and awaits worker termination cleanly.
// It is fully idempotent and safe to call multiple times.
func (notifier *Notifier) Stop() {
	notifier.stopOnce.Do(func() {
		notifier.mu.Lock()
		notifier.isStopping = true
		notifier.mu.Unlock()

		// 1. Signal processMessage to finish draining NotifChan into sender queues and exit
		close(notifier.shutdown)
		notifier.processWg.Wait()

		// 2. Close each sender queue so workers drain their pending notifications
		notifier.mu.Lock()
		for tag := range notifier.senderQueues {
			notifier.stopSenderLocked(tag)
		}
		notifier.mu.Unlock()

		// 3. Wait for all sender workers to deliver and exit
		notifier.workersWg.Wait()
	})
}

// -----------------------------------------------------------------------------

// RegisterSender registers a custom or programmatic notification sender with its dedicated worker.
func (notifier *Notifier) RegisterSender(sender interfaces.INotifSender) {
	if isNilSender(sender) {
		notifier.Logger.Error("RegisterSender called with nil sender")
		return
	}

	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	if notifier.isStopping {
		return
	}

	tag := sender.GetTag()
	var prevDone chan struct{}
	if _, exists := notifier.senderQueues[tag]; exists {
		notifier.Logger.Info("Sender '%s' already registered. Gracefully replacing previous worker...", tag)
		prevDone = notifier.stopSenderLocked(tag)
	}

	notifier.TagToSenderMap[tag] = sender

	// Designate as default main provider if it is telegram or if no default provider is set yet
	if tag == "telegram" || notifier.DefaultSender == nil {
		notifier.DefaultSender = sender
		notifier.DefaultTag = tag
	}

	// Automatically map the sender's default log level to levelToTags
	if lvl := sender.GetLogLevel(); lvl != "" {
		for _, l := range strings.Split(lvl, ",") {
			l = strings.TrimSpace(strings.ToUpper(l))
			if l != "" {
				notifier.addLevelTagLocked(l, tag)
			}
		}
	}

	notifier.startSenderWorkerLocked(tag, sender, prevDone)
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) startSenderWorker(tag string, sender interfaces.INotifSender, queue chan *utils.NotifMessage, metrics *senderMetrics) {
	if isNilSender(sender) {
		return
	}
	if tag == "" {
		tag = sender.GetTag()
	}
	for msg := range queue {
		// Each dispatch has a 30s timeout to prevent hanging the worker
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := sender.SendMessage(ctx, msg.Message, msg.Attachment, notifier.Name); err != nil {
			if metrics != nil {
				metrics.recordFailure(err)
			}
			notifier.Logger.Error("[%s] Dispatch failed: %v", tag, err)
			// If an auxiliary provider fails, notify the default main provider as fallback
			notifier.mu.RLock()
			defaultSender := notifier.DefaultSender
			defaultTag := notifier.DefaultTag
			defaultMetrics := notifier.senderMetrics[defaultTag]
			notifier.mu.RUnlock()

			if defaultSender != nil && tag != defaultTag && msg.Level != "FALLBACK" {
				fallbackText := fmt.Sprintf("⚠️ [Notification Fallback] Provider '%s' failed to deliver alert: %v\nOriginal message: %s", tag, err, msg.Message)
				if fbErr := defaultSender.SendMessage(ctx, fallbackText, msg.Attachment, "fallback"); fbErr != nil {
					if defaultMetrics != nil {
						defaultMetrics.recordFailure(fbErr)
					}
					notifier.Logger.Error("[%s] Fallback alert delivery to %s failed: %v", tag, defaultTag, fbErr)
				} else {
					if defaultMetrics != nil {
						defaultMetrics.recordSuccess()
					}
					notifier.Logger.Info("Dispatched fallback alert to %s for failed provider '%s'", defaultTag, tag)
				}
			}
		} else if metrics != nil {
			metrics.recordSuccess()
		}
		cancel()
	}
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) processMessage() {
	defer notifier.processWg.Done()
	for {
		select {
		case recvNotifMessage := <-notifier.NotifChan:
			if recvNotifMessage != nil {
				notifier.dispatchMessage(recvNotifMessage)
			}
		case <-notifier.shutdown:
			// Drain remaining messages in NotifChan into per-platform queues before exiting
			for {
				select {
				case recvNotifMessage := <-notifier.NotifChan:
					if recvNotifMessage != nil {
						notifier.dispatchMessage(recvNotifMessage)
					}
				default:
					return
				}
			}
		}
	}
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) dispatchMessage(recvNotifMessage *utils.NotifMessage) {
	// Recursion prevention: Never process internal drop/failure logs as alerts [NS-1]
	if strings.Contains(recvNotifMessage.Message, "Notification dropped:") || strings.Contains(recvNotifMessage.Message, "No worker queue found") {
		return
	}

	msgCopy := *recvNotifMessage

	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	targetTags := make(map[string]bool)

	// 1. Explicit tags (only include tags that have active senders)
	for _, tag := range msgCopy.Tags {
		if _, ok := notifier.TagToSenderMap[tag]; ok {
			targetTags[tag] = true
		}
	}

	// 2. Implicit tags from Level (only include tags that have active senders)
	if msgCopy.Level != "" {
		levelKey := strings.ToUpper(msgCopy.Level)
		if tags, ok := notifier.levelToTags[levelKey]; ok {
			notifier.Logger.Debug("Level-based implicit routing: %s -> %v", levelKey, tags)
			for _, tag := range tags {
				if _, ok := notifier.TagToSenderMap[tag]; ok {
					targetTags[tag] = true
				}
			}
		}
	}

	// 3. Default routing (FEAT-001 Scenario 3):
	// If no routing tags matched (e.g. untagged alert, unmapped level, or unmapped tags),
	// route solely to the default main provider and visually mark the fallback directly in the message.
	if len(targetTags) == 0 {
		if notifier.DefaultTag != "" {
			notifier.Logger.Info("No specific provider match for message (level: '%s', tags: %v); routing to default provider '%s'", msgCopy.Level, msgCopy.Tags, notifier.DefaultTag)
			targetTags[notifier.DefaultTag] = true

			// Prepend visual indicator in the message so it is clearly visible in the notification
			reason := "Untagged Alert"
			if len(msgCopy.Tags) > 0 {
				reason = fmt.Sprintf("Unmapped Tags: %v", msgCopy.Tags)
			} else if msgCopy.Level != "" {
				reason = fmt.Sprintf("Unmapped Level: %s", msgCopy.Level)
			}
			if !strings.HasPrefix(msgCopy.Message, "⚠️ [Default Fallback") {
				msgCopy.Message = fmt.Sprintf("⚠️ [Default Fallback - %s]\n%s", reason, msgCopy.Message)
			}
		} else {
			notifier.Logger.Warning("Notification dropped: no active default sender configured to receive alert: %s", msgCopy.Message)
		}
	}

	// Loop prevention: Never route a provider's own dispatch failure log back to that provider
	for tag := range targetTags {
		if strings.HasPrefix(msgCopy.Message, "["+tag+"]") {
			delete(targetTags, tag)
		}
	}

	notifier.Logger.Debug("Processing message with final tags: %v", targetTags)

	// 4. Dispatch to per-platform worker queues
	for tag := range targetTags {
		if queue, ok := notifier.senderQueues[tag]; ok {
			select {
			case queue <- &msgCopy:
			default:
				if m, ok := notifier.senderMetrics[tag]; ok {
					m.recordDrop()
				}
				notifier.Logger.Warning("[%s] Worker queue full! Dropping notification to prevent OOM.", tag)
			}
		} else {
			notifier.Logger.Warning("No worker queue found for tag: %s", tag)
		}
	}
}

// -----------------------------------------------------------------------------

// GetDefaultSender returns the primary/default notification sender.
func (notifier *Notifier) GetDefaultSender() interfaces.INotifSender {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()
	return notifier.DefaultSender
}

// -----------------------------------------------------------------------------

// GetDefaultTag returns the tag of the primary/default notification sender.
func (notifier *Notifier) GetDefaultTag() string {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()
	return notifier.DefaultTag
}
