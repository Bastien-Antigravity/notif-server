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
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Bastien-Antigravity/notif-server/src/interfaces"
	"github.com/Bastien-Antigravity/notif-server/src/notifiers"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// Notifier is the concrete implementation of the notification service.
type Notifier struct {
	proto_msg.UnimplementedNotifServiceServer
	Name           string
	appConfig      *toolbox_config.AppConfig
	Logger         log_interfaces.Logger
	TagToSenderMap map[string]interfaces.INotifSender
	DefaultSender  interfaces.INotifSender
	DefaultTag     string
	NotifChan      chan *utils.NotifMessage
	senderQueues   map[string]chan *utils.NotifMessage
	currentConf    map[string]map[string]string
	levelToTags    map[string][]string // Implicit routing map: Level -> [Tag1, Tag2]
	mu             sync.RWMutex
	shutdown       chan struct{}
	OnUpdate       func()
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

	go curNotifier.processMessage()

	return curNotifier
}

// -----------------------------------------------------------------------------

// InitDefaultProviders initializes baseline providers (like telegram) from static capabilities
// if no dynamic providers have been registered for them.
func (notifier *Notifier) InitDefaultProviders() {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

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
		notifier.startSenderLocked("TELEGRAM", "telegram", conf)

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
			if existed {
				notifier.Logger.Info("Updating configuration for notifier: %s", sectionName)
				notifier.stopSenderLocked(sectionName)
			} else {
				notifier.Logger.Info("Initializing new notifier: %s (Type: %s)", sectionName, platType)
			}
			notifier.startSenderLocked(platType, sectionName, sectionConf)
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

	notifier.currentConf = newConf
	notifier.levelToTags = newLevelToTags
	notifier.Logger.Info("Notifier routing map updated: %v", notifier.levelToTags)
	if notifier.OnUpdate != nil {
		go notifier.OnUpdate()
	}
}

// -----------------------------------------------------------------------------

// NotifierStatus contains basic info about an active provider
type NotifierStatus struct {
	Name    string
	Type    string
	Healthy bool
}

// -----------------------------------------------------------------------------

// GetActiveNotifiers returns information about all currently active notification senders.
func (notifier *Notifier) GetActiveNotifiers() []NotifierStatus {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	result := make([]NotifierStatus, 0, len(notifier.TagToSenderMap))
	for tag, sender := range notifier.TagToSenderMap {
		result = append(result, NotifierStatus{
			Name:    tag,
			Type:    reflect.TypeOf(sender).String(),
			Healthy: true,
		})
	}
	return result
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) stopSenderLocked(tag string) {
	if queue, ok := notifier.senderQueues[tag]; ok {
		close(queue)
		delete(notifier.senderQueues, tag)
	}
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
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) startSenderLocked(platform, tag string, conf map[string]string) {
	var sender interfaces.INotifSender
	var err error

	decrypt := notifier.appConfig.DecryptSecret

	switch platform {
	case "TELEGRAM":
		sender, err = notifiers.NewTelegramSender(conf, tag, notifier.Logger, decrypt)
	case "DISCORD":
		sender, err = notifiers.NewDiscordSender(conf, tag, notifier.Logger, decrypt)
	case "MATRIX":
		sender, err = notifiers.NewMatrixSender(conf, tag, notifier.Logger, decrypt)
	case "GMAIL":
		sender, err = notifiers.NewGmailSender(conf, tag, notifier.Logger, decrypt)
	default:
		notifier.Logger.Error("Unsupported notification platform '%s' for provider '%s'", platform, tag)
		return
	}

	if err != nil {
		notifier.Logger.Error("Failed to initialize %s (%s): %v", platform, tag, err)
		return
	}
	// If sender == nil and err == nil, provider is unconfigured (configuration is optional)
	if sender == nil {
		return
	}

	notifier.TagToSenderMap[tag] = sender

	// Designate as default main provider if it is telegram or if no default provider is set yet
	if tag == "telegram" || notifier.DefaultSender == nil {
		notifier.DefaultSender = sender
		notifier.DefaultTag = tag
	}

	// Single worker per sender ensures sequential message ordering and avoids rate-limit violations
	queue := make(chan *utils.NotifMessage, 100)
	notifier.senderQueues[tag] = queue

	go notifier.startSenderWorker(tag, queue)
}

// -----------------------------------------------------------------------------

// Notify sends a structured notification message.
func (notifier *Notifier) Notify(msg *utils.NotifMessage) error {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	select {
	case notifier.NotifChan <- msg:
		return nil
	default:
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

// Stop shuts down the notifier and all active workers.
func (notifier *Notifier) Stop() {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	close(notifier.shutdown)
	for tag := range notifier.senderQueues {
		notifier.stopSenderLocked(tag)
	}
}

// -----------------------------------------------------------------------------

// RegisterSender registers a custom or programmatic notification sender with its dedicated worker.
func (notifier *Notifier) RegisterSender(sender interfaces.INotifSender) {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	tag := sender.GetTag()
	notifier.TagToSenderMap[tag] = sender

	// Designate as default main provider if it is telegram or if no default provider is set yet
	if tag == "telegram" || notifier.DefaultSender == nil {
		notifier.DefaultSender = sender
		notifier.DefaultTag = tag
	}

	queue := make(chan *utils.NotifMessage, 100)
	notifier.senderQueues[tag] = queue

	// Automatically map the sender's default log level to levelToTags
	if lvl := sender.GetLogLevel(); lvl != "" {
		for _, l := range strings.Split(lvl, ",") {
			l = strings.TrimSpace(strings.ToUpper(l))
			if l != "" {
				notifier.addLevelTagLocked(l, tag)
			}
		}
	}

	go notifier.startSenderWorker(tag, queue)
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) startSenderWorker(tag string, queue chan *utils.NotifMessage) {
	for {
		select {
		case msg, ok := <-queue:
			if !ok {
				return // Channel closed via stopSenderLocked, terminate worker cleanly
			}
			notifier.mu.RLock()
			sender := notifier.TagToSenderMap[tag]
			defaultSender := notifier.DefaultSender
			defaultTag := notifier.DefaultTag
			notifier.mu.RUnlock()
			if sender == nil {
				continue
			}

			// Each dispatch has a 30s timeout to prevent hanging the worker
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := sender.SendMessage(ctx, msg.Message, msg.Attachment, notifier.Name); err != nil {
				notifier.Logger.Error("[%s] Dispatch failed: %v", tag, err)
				// If an auxiliary provider fails, notify the default main provider as fallback
				if defaultSender != nil && tag != defaultTag && msg.Level != "FALLBACK" {
					fallbackText := fmt.Sprintf("⚠️ [Notification Fallback] Provider '%s' failed to deliver alert: %v\nOriginal message: %s", tag, err, msg.Message)
					if fbErr := defaultSender.SendMessage(ctx, fallbackText, msg.Attachment, "fallback"); fbErr != nil {
						notifier.Logger.Error("[%s] Fallback alert delivery to %s failed: %v", tag, defaultTag, fbErr)
					} else {
						notifier.Logger.Info("Dispatched fallback alert to %s for failed provider '%s'", defaultTag, tag)
					}
				}
			}
			cancel()
		case <-notifier.shutdown:
			return
		}
	}
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) processMessage() {
	for {
		select {
		case recvNotifMessage := <-notifier.NotifChan:
			if recvNotifMessage == nil {
				continue
			}

			notifier.mu.RLock()

			targetTags := make(map[string]bool)

			// 1. Explicit tags (only include tags that have active senders)
			for _, tag := range recvNotifMessage.Tags {
				if _, ok := notifier.TagToSenderMap[tag]; ok {
					targetTags[tag] = true
				}
			}

			// 2. Implicit tags from Level (only include tags that have active senders)
			if recvNotifMessage.Level != "" {
				levelKey := strings.ToUpper(recvNotifMessage.Level)
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
					notifier.Logger.Info("No specific provider match for message (level: '%s', tags: %v); routing to default provider '%s'", recvNotifMessage.Level, recvNotifMessage.Tags, notifier.DefaultTag)
					targetTags[notifier.DefaultTag] = true

					// Prepend visual indicator in the message so it is clearly visible in the notification
					reason := "Untagged Alert"
					if len(recvNotifMessage.Tags) > 0 {
						reason = fmt.Sprintf("Unmapped Tags: %v", recvNotifMessage.Tags)
					} else if recvNotifMessage.Level != "" {
						reason = fmt.Sprintf("Unmapped Level: %s", recvNotifMessage.Level)
					}
					if !strings.HasPrefix(recvNotifMessage.Message, "⚠️ [Default Fallback") {
						recvNotifMessage.Message = fmt.Sprintf("⚠️ [Default Fallback - %s]\n%s", reason, recvNotifMessage.Message)
					}
				} else {
					notifier.Logger.Warning("Notification dropped: no active default sender configured to receive alert: %s", recvNotifMessage.Message)
				}
			}

			// Loop prevention: Never route a provider's own dispatch failure log back to that provider
			for tag := range targetTags {
				if strings.HasPrefix(recvNotifMessage.Message, "["+tag+"]") {
					delete(targetTags, tag)
				}
			}

			notifier.Logger.Debug("Processing message with final tags: %v", targetTags)

			// 4. Dispatch to per-platform worker queues
			for tag := range targetTags {
				if queue, ok := notifier.senderQueues[tag]; ok {
					select {
					case queue <- recvNotifMessage:
					default:
						notifier.Logger.Warning("[%s] Worker queue full! Dropping notification to prevent OOM.", tag)
					}
				} else {
					notifier.Logger.Warning("No worker queue found for tag: %s", tag)
				}
			}
			notifier.mu.RUnlock()
		case <-notifier.shutdown:
			return
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


