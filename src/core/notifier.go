package notifier

/*
ESSENTIAL PROCESS:
Manages the core notification dispatch logic, worker pools, and asynchronous queuing.
Ensures that slow external APIs do not block the ingestion layer.

DATA FLOW:
1. Ingestion layer (TCP/gRPC) pushes messages to NotifChan.
2. Router identifies target platforms based on message tags.
3. Message is pushed to platform-specific buffered worker queues.
4. Workers pick up messages and execute delivery with a 30s timeout.

KEY PARAMETERS:
- NotifChan: Primary ingestion channel for structured messages.
- RawNotifChan: Ingestion channel for raw serialized binary data.
- senderQueues: Map of platform tags to their respective worker queues.
*/

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Bastien-Antigravity/notif-server/src/interfaces"
	"github.com/Bastien-Antigravity/notif-server/src/notifiers"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// Notifier is the concrete implementation of the notification service.
type Notifier struct {
	proto_msg.UnimplementedNotifServiceServer
	Name           string
	config         *distributed_config.Config
	Logger         log_interfaces.Logger
	TagToSenderMap map[string]interfaces.INotifSender
	NotifChan      chan *utils.NotifMessage
	RawNotifChan   chan []byte
	senderQueues   map[string]chan *utils.NotifMessage
	senderShutdown map[string]chan struct{} // Per-platform shutdown signal
	currentConf    map[string]map[string]string
	levelToTags    map[string][]string // Implicit routing map: Level -> [Tag1, Tag2]
	mu             sync.RWMutex
	shutdown       chan struct{}
}

// -----------------------------------------------------------------------------

// NewNotifier creates a new instance of the notification service.
func NewNotifier(conf *distributed_config.Config, logger log_interfaces.Logger, parentName string) *Notifier {
	curNotifier := &Notifier{
		Name:           parentName,
		config:         conf,
		Logger:         logger,
		NotifChan:      make(chan *utils.NotifMessage, 100),
		RawNotifChan:   make(chan []byte, 100),
		TagToSenderMap: make(map[string]interfaces.INotifSender),
		senderQueues:   make(map[string]chan *utils.NotifMessage),
		senderShutdown: make(map[string]chan struct{}),
		currentConf:    make(map[string]map[string]string),
		levelToTags:    make(map[string][]string),
		shutdown:       make(chan struct{}),
	}

	curNotifier.EnsureSafeLogger()

	// 1. Initial Load
	curNotifier.Reload(*conf.LiveConfig.Load())

	// 2. Register for Live Updates
	conf.OnLiveConfUpdate(func(newConf map[string]map[string]string) {
		curNotifier.Logger.Info("Live configuration update received. Reloading senders...")
		curNotifier.Reload(newConf)
	})

	go curNotifier.processMessage()
	go curNotifier.ConsumeRawMessages()

	return curNotifier
}

// -----------------------------------------------------------------------------

// Reload compares the new configuration with the current state and hot-swaps
// senders that have changed or were added/removed.
func (notifier *Notifier) Reload(newConf map[string]map[string]string) {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	platforms := []string{"TELEGRAM", "DISCORD", "MATRIX", "GMAIL"}
	newLevelToTags := make(map[string][]string)

	for _, platform := range platforms {
		newPlatConf, ok := newConf[platform]
		oldPlatConf, existed := notifier.currentConf[platform]

		// Scenario A: Provider was removed
		if !ok && existed {
			if notifier.Logger != nil {
				notifier.Logger.Info("Removing notification provider: %s", platform)
			}
			notifier.stopSender(platform)
			continue
		}

		// Scenario B: Provider added or updated
		if ok && (!existed || !reflect.DeepEqual(newPlatConf, oldPlatConf)) {
			if existed {
				if notifier.Logger != nil {
					notifier.Logger.Info("Updating configuration for provider: %s", platform)
				}
				notifier.stopSender(platform)
			} else {
				if notifier.Logger != nil {
					notifier.Logger.Info("Initializing new provider: %s", platform)
				}
			}
			notifier.startSender(platform, newPlatConf)
		}

		// Re-build implicit routing map for this platform if it's active
		if ok {
			if logLevels, ok := newPlatConf["LOGLEVEL"]; ok {
				tag := platform
				if t, ok := newPlatConf["TAG"]; ok {
					tag = t
				}
				for _, level := range strings.Split(logLevels, ",") {
					level = strings.TrimSpace(strings.ToUpper(level))
					if level != "" {
						newLevelToTags[level] = append(newLevelToTags[level], tag)
					}
				}
			}
		}
	}

	notifier.currentConf = newConf
	notifier.levelToTags = newLevelToTags
	if notifier.Logger != nil {
		notifier.Logger.Info("Implicit routing map updated: %v", notifier.levelToTags)
	}
}

func (notifier *Notifier) stopSender(tag string) {
	if signal, ok := notifier.senderShutdown[tag]; ok {
		close(signal)
		delete(notifier.senderShutdown, tag)
	}
	delete(notifier.TagToSenderMap, tag)
	delete(notifier.senderQueues, tag)
}

func (notifier *Notifier) startSender(platform string, conf map[string]string) {
	var sender interfaces.INotifSender
	var err string

	switch platform {
	case "TELEGRAM":
		sender, err = notifiers.NewTelegramSender(conf, "TELEGRAM")
	case "DISCORD":
		sender, err = notifiers.NewDiscordSender(conf, "DISCORD")
	case "MATRIX":
		sender, err = notifiers.NewMatrixSender(conf, "MATRIX")
	case "GMAIL":
		sender, err = notifiers.NewGmailSender(conf, "GMAIL")
	}

	if err != "" {
		if notifier.Logger != nil {
			notifier.Logger.Error("Failed to initialize %s: %s", platform, err)
		}
		return
	}

	tag := sender.GetTag()
	notifier.TagToSenderMap[tag] = sender

	// Create buffered queue and shutdown signal
	queue := make(chan *utils.NotifMessage, 1000)
	notifier.senderQueues[tag] = queue
	
	shutdown := make(chan struct{})
	notifier.senderShutdown[tag] = shutdown

	// Start worker pool (5 workers)
	for i := 0; i < 5; i++ {
		go notifier.startSenderWorker(tag, sender, queue, shutdown)
	}
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

// SendRaw sends a raw byte message (serialized).
func (notifier *Notifier) SendRaw(data []byte) error {
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()

	select {
	case notifier.RawNotifChan <- data:
		return nil
	default:
		return context.DeadlineExceeded // Buffer full
	}
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

// LoadNotifSender is now a wrapper around Reload for backward compatibility
// but essentially Reload is the new standard.
func (notifier *Notifier) LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string {
	notifier.Reload(notifiersConf)
	return nil // Note: Implicit routing return will be handled in Phase 2
}

// -----------------------------------------------------------------------------

// Stop shuts down the notifier and all active workers.
func (notifier *Notifier) Stop() {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	close(notifier.shutdown)
	for tag := range notifier.senderShutdown {
		notifier.stopSender(tag)
	}
}

// -----------------------------------------------------------------------------

// RegisterMockSender is a helper for testing to manually register a sender with its worker pool.
func (notifier *Notifier) RegisterMockSender(sender interfaces.INotifSender) {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	tag := sender.GetTag()
	notifier.TagToSenderMap[tag] = sender

	queue := make(chan *utils.NotifMessage, 1000)
	notifier.senderQueues[tag] = queue

	shutdown := make(chan struct{})
	notifier.senderShutdown[tag] = shutdown

	for i := 0; i < 5; i++ {
		go notifier.startSenderWorker(tag, sender, queue, shutdown)
	}
}

// -----------------------------------------------------------------------------

// EnsureSafeLogger ensures the logger is initialized with proper metadata.
func (notifier *Notifier) EnsureSafeLogger() {
	if notifier.Logger != nil {
		notifier.Logger.AddMetadata("component", "notifier")
	}
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) ConsumeRawMessages() {
	for {
		select {
		case rawData := <-notifier.RawNotifChan:
			if notifier.Logger != nil {
				notifier.Logger.Debug("Received raw message, size: %d", len(rawData))
			}
			msg, err := DeserializeNotifMsg(rawData)
			if err != nil {
				if notifier.Logger != nil {
					notifier.Logger.Error("Error deserializing raw message: %v", err)
				}
				continue
			}
			if notifier.Logger != nil {
				notifier.Logger.Debug("Deserialized message: %+v", msg)
			}
			notifier.NotifChan <- msg
		case <-notifier.shutdown:
			return
		}
	}
}

// -----------------------------------------------------------------------------

func (notifier *Notifier) startSenderWorker(tag string, sender interfaces.INotifSender, queue chan *utils.NotifMessage, shutdown chan struct{}) {
	for {
		select {
		case msg := <-queue:
			// Each dispatch has a 30s timeout to prevent hanging the worker
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := sender.SendMessage(ctx, msg.Message, msg.Attachment, notifier.Name); err != nil {
				if notifier.Logger != nil {
					notifier.Logger.Error("[%s] Dispatch failed: %v", tag, err)
				}
			}
			cancel()
		case <-shutdown:
			// Draining logic could be added here if needed
			return
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
			notifier.mu.RLock()
			
			// 1. Implicit Routing: Apply Level mapping
			targetTags := make(map[string]bool)
			
			// Add explicit tags
			for _, tag := range recvNotifMessage.Tags {
				targetTags[tag] = true
			}
			
			// Add implicit tags from Level
			if recvNotifMessage.Level != "" {
				levelKey := strings.ToUpper(recvNotifMessage.Level)
				if tags, ok := notifier.levelToTags[levelKey]; ok {
					if notifier.Logger != nil {
						notifier.Logger.Debug("Level-based implicit routing: %s -> %v", levelKey, tags)
					}
					for _, tag := range tags {
						targetTags[tag] = true
					}
				}
			}

			if notifier.Logger != nil {
				notifier.Logger.Debug("Processing message with final tags: %v", targetTags)
			}

			// 2. Dispatch to worker queues
			for tag := range targetTags {
				if queue, ok := notifier.senderQueues[tag]; ok {
					if notifier.Logger != nil {
						notifier.Logger.Debug("Pushing message to queue for tag: %s", tag)
					}
					// Non-blocking dispatch to worker queue
					select {
					case queue <- recvNotifMessage:
						// Queued successfully
					default:
						if notifier.Logger != nil {
							notifier.Logger.Warning("[%s] Worker queue full! Dropping notification to prevent OOM.", tag)
						}
					}
				} else {
					if notifier.Logger != nil {
						notifier.Logger.Warning("No worker queue found for tag: %s", tag)
					}
				}
			}
			notifier.mu.RUnlock()
		case <-notifier.shutdown:
			return
		}
	}
}
