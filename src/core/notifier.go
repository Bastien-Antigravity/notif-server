package notifier

import (
	"context"
	"strings"
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
	TagToSenderMap map[string]interfaces.NotifSenderInterface
	NotifChan      chan *utils.NotifMessage
	RawNotifChan   chan []byte
	senderQueues   map[string]chan *utils.NotifMessage
	shutdown       chan struct{}
}

// NewNotifier creates a new instance of the notification service.
func NewNotifier(conf *distributed_config.Config, logger log_interfaces.Logger, parentName string) *Notifier {
	curNotifier := &Notifier{
		Name:           parentName,
		config:         conf,
		Logger:         logger,
		NotifChan:      make(chan *utils.NotifMessage, 100),
		RawNotifChan:   make(chan []byte, 100),
		TagToSenderMap: make(map[string]interfaces.NotifSenderInterface),
		senderQueues:   make(map[string]chan *utils.NotifMessage),
		shutdown:       make(chan struct{}),
	}

	if curNotifier.Logger != nil {
		curNotifier.Logger.AddMetadata("component", "notifier")
	}

	// Load initial config and initialize workers
	curNotifier.LoadNotifSender(*conf.LiveConfig.Load())

	go curNotifier.processMessage()
	go curNotifier.ConsumeRawMessages()
	
	return curNotifier
}

// ConsumeRawMessages handles Cap'n Proto binary ingestion.
func (notifier *Notifier) ConsumeRawMessages() {
	for {
		select {
		case rawData := <-notifier.RawNotifChan:
			msg, err := DeserializeNotifMsg(rawData)
			if err != nil {
				if notifier.Logger != nil {
					notifier.Logger.Error("Error deserializing raw message: %v", err)
				}
				continue
			}
			notifier.NotifChan <- msg
		case <-notifier.shutdown:
			return
		}
	}
}

// LoadNotifSender initializes senders and their corresponding worker pools.
func (notifier *Notifier) LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string {
	returnLogLevelByTag := map[string][]string{}

	// Helper to register sender and start workers
	register := func(sender interfaces.NotifSenderInterface, logLevels string, tag string) {
		for _, level := range strings.Split(logLevels, ",") {
			level = strings.TrimSpace(level)
			returnLogLevelByTag[level] = append(returnLogLevelByTag[level], tag)
		}
		
		notifier.TagToSenderMap[tag] = sender
		
		// Create buffered queue for this sender (1000 messages)
		queue := make(chan *utils.NotifMessage, 1000)
		notifier.senderQueues[tag] = queue
		
		// Start worker pool (5 workers per platform)
		for i := 0; i < 5; i++ {
			go notifier.startSenderWorker(tag, sender, queue)
		}
	}

	if conf, ok := notifiersConf["TELEGRAM"]; ok {
		if s, err := notifiers.NewTelegramSender(conf, "TELEGRAM"); err == "" {
			register(s, s.GetLogLevel(), s.GetTag())
		} else if notifier.Logger != nil {
			notifier.Logger.Error("Error loading Telegram sender: %s", err)
		}
	}

	if conf, ok := notifiersConf["DISCORD"]; ok {
		if s, err := notifiers.NewDiscordSender(conf, "DISCORD"); err == "" {
			register(s, s.GetLogLevel(), s.GetTag())
		} else if notifier.Logger != nil {
			notifier.Logger.Error("Error loading Discord sender: %s", err)
		}
	}

	if conf, ok := notifiersConf["MATRIX"]; ok {
		if s, err := notifiers.NewMatrixSender(conf, "MATRIX"); err == "" {
			register(s, s.GetLogLevel(), s.GetTag())
		} else if notifier.Logger != nil {
			notifier.Logger.Error("Error loading Matrix sender: %s", err)
		}
	}

	if conf, ok := notifiersConf["GMAIL"]; ok {
		if s, err := notifiers.NewGmailSender(conf, "GMAIL"); err == "" {
			register(s, s.GetLogLevel(), s.GetTag())
		} else if notifier.Logger != nil {
			notifier.Logger.Error("Error loading Gmail sender: %s", err)
		}
	}

	return returnLogLevelByTag
}

func (notifier *Notifier) startSenderWorker(tag string, sender interfaces.NotifSenderInterface, queue chan *utils.NotifMessage) {
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
		case <-notifier.shutdown:
			return
		}
	}
}

func (notifier *Notifier) processMessage() {
	for {
		select {
		case recvNotifMessage := <-notifier.NotifChan:
			for _, tag := range recvNotifMessage.Tags {
				if queue, ok := notifier.senderQueues[tag]; ok {
					// Non-blocking dispatch to worker queue
					select {
					case queue <- recvNotifMessage:
						// Queued successfully
					default:
						if notifier.Logger != nil {
							notifier.Logger.Warning("[%s] Worker queue full! Dropping notification to prevent OOM.", tag)
						}
					}
				}
			}
		case <-notifier.shutdown:
			return
		}
	}
}

// Notify sends a structured notification message.
func (notifier *Notifier) Notify(msg *utils.NotifMessage) error {
	select {
	case notifier.NotifChan <- msg:
		return nil
	default:
		return context.DeadlineExceeded // Buffer full
	}
}

// SendRaw sends a raw byte message (serialized).
func (notifier *Notifier) SendRaw(data []byte) error {
	select {
	case notifier.RawNotifChan <- data:
		return nil
	default:
		return context.DeadlineExceeded // Buffer full
	}
}

// SendNotification implements proto_msg.NotifServiceServer (gRPC)
func (notifier *Notifier) SendNotification(ctx context.Context, req *proto_msg.NotifRequest) (*proto_msg.NotifResponse, error) {
	msg := &utils.NotifMessage{
		Message:    req.Message,
		Tags:       req.Tags,
		Attachment: req.Attachment,
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

func (notifier *Notifier) Stop() {
	close(notifier.shutdown)
}
