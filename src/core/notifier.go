package notifier

import (
	"strings"

	"github.com/Bastien-Antigravity/notif-server/src/interfaces"
	"github.com/Bastien-Antigravity/notif-server/src/notifiers"
	proto_msg "github.com/Bastien-Antigravity/notif-server/src/schemas/protobuf"
	"context"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// ce que doit faire le Notifier :
//
// soit instancié dans le logger (withNotifServer = false)
// 		- chargement des notifs serveur selon la config (notif par défaut ajouté si rien dans la config ??)
// 		- le logger place les notifs Message dans la NotifChan
// 		- processMessage (goroutine) récupére les messages et les envoi (selon le tag)
// soit instancié dans le notif_server
// 		- instanciation avec le notif_server, même fonctionnement que l'instanciation avec le logger
//		- dans ce cas le logger n'utilise pas cette class...
//      - le logger utilise sa propre NotifChan et son 'serializer socket' pour envoyer les messages au notif_server...
//

/////////////////////////////////
// Notifier is the concrete implementation of the notification service.
type Notifier struct {
	proto_msg.UnimplementedNotifServiceServer
	Name           string
	config         *distributed_config.Config
	Logger         log_interfaces.Logger
	TagToSenderMap map[string]interfaces.NotifSenderInterface
	NotifChan      chan *utils.NotifMessage
	RawNotifChan   chan []byte
}

// NewNotifier creates a new instance of the notification service.
func NewNotifier(conf *distributed_config.Config, logger log_interfaces.Logger, parentName string) *Notifier {
	curNotifier := &Notifier{
		Name:           parentName,
		config:         conf,
		Logger:         logger,
		NotifChan:      make(chan *utils.NotifMessage),
		RawNotifChan:   make(chan []byte),
		TagToSenderMap: make(map[string]interfaces.NotifSenderInterface),
	}

	// Add identification metadata
	if curNotifier.Logger != nil {
		curNotifier.Logger.AddMetadata("component", "notifier")
	}

	// Load initial config
	curNotifier.LoadNotifSender(conf.LiveConfig)

	go curNotifier.processMessage()
	go curNotifier.ConsumeRawMessages()
	return curNotifier
}

func (notifier *Notifier) ConsumeRawMessages() {
	for rawData := range notifier.RawNotifChan {
		msg, err := DeserializeNotifMsg(rawData)
		if err != nil {
			if notifier.Logger != nil {
				notifier.Logger.Error("Error deserializing raw message: %v", err)
			}
			continue
		}
		notifier.NotifChan <- msg
	}
}

func (notifier *Notifier) LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string {
	// ... (implementation remains same but uses notifier receiver)
	returnLogLevelByTag := map[string][]string{}

	if confTele, ok := notifiersConf["TELEGRAM"]; ok {
		telegram, telegramErrorList := notifiers.NewTelegramSender(confTele, "TELEGRAM")
		if telegramErrorList != "" {
			if notifier.Logger != nil {
				notifier.Logger.Error("Error loading Telegram sender: %s", telegramErrorList)
			}
		} else {
			for _, logLevel := range strings.Split(telegram.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, telegram.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{telegram.GetTag()}
				}
			}
			notifier.TagToSenderMap[telegram.GetTag()] = telegram
		}
	}

	if confDisco, ok := notifiersConf["DISCORD"]; ok {
		discord, discordErrorList := notifiers.NewDiscordSender(confDisco, "DISCORD")
		if discordErrorList != "" {
			if notifier.Logger != nil {
				notifier.Logger.Error("Error loading Discord sender: %s", discordErrorList)
			}
		} else {
			for _, logLevel := range strings.Split(discord.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, discord.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{discord.GetTag()}
				}
			}
			notifier.TagToSenderMap[discord.GetTag()] = discord
		}
	}

	if confMatrix, ok := notifiersConf["MATRIX"]; ok {
		matrix, matrixErrorList := notifiers.NewMatrixSender(confMatrix, "MATRIX")
		if matrixErrorList != "" {
			if notifier.Logger != nil {
				notifier.Logger.Error("Error loading Matrix sender: %s", matrixErrorList)
			}
		} else {
			for _, logLevel := range strings.Split(matrix.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, matrix.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{matrix.GetTag()}
				}
			}
			notifier.TagToSenderMap[matrix.GetTag()] = matrix
		}
	}

	if confGmail, ok := notifiersConf["GMAIL"]; ok {
		gmail, gmailErrorList := notifiers.NewGmailSender(confGmail, "GMAIL")
		if gmailErrorList != "" {
			if notifier.Logger != nil {
				notifier.Logger.Error("Error loading Gmail sender: %s", gmailErrorList)
			}
		} else {
			for _, logLevel := range strings.Split(gmail.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, gmail.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{gmail.GetTag()}
				}
			}
			notifier.TagToSenderMap[gmail.GetTag()] = gmail
		}
	}

	return returnLogLevelByTag
}

// Notify sends a structured notification message.
func (notifier *Notifier) Notify(msg *utils.NotifMessage) error {
	notifier.NotifChan <- msg
	return nil
}

// SendRaw sends a raw byte message (serialized).
func (notifier *Notifier) SendRaw(data []byte) error {
	notifier.RawNotifChan <- data
	return nil
}

func (notifier *Notifier) processMessage() {
	for recvNotifMessage := range notifier.NotifChan {
		for _, tag := range recvNotifMessage.Tags {
			if sender, ok := notifier.TagToSenderMap[tag]; ok {
				go sender.SendMessage(recvNotifMessage.Message, recvNotifMessage.Attachment, notifier.Name)
			}
		}
	}
}

// SendNotification implements proto_msg.NotifServiceServer
func (notifier *Notifier) SendNotification(ctx context.Context, req *proto_msg.NotifRequest) (*proto_msg.NotifResponse, error) {
	msg := &utils.NotifMessage{
		Message:    req.Message,
		Tags:       req.Tags,
		Attachment: req.Attachment,
	}
	notifier.NotifChan <- msg
	return &proto_msg.NotifResponse{Success: true}, nil
}

// Notifier
/////////////////////////////////
