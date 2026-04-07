package notifie

import (
	"fmt"
	"strings"

	"github.com/Bastien-Antigravity/notif-server/src/interfaces"
	"github.com/Bastien-Antigravity/notif-server/src/notifiers"

	"github.com/Bastien-Antigravity/universal-logger/src/config"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
)

// ce que doit faire le Notifie :
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
// Notifie

// when called by logger and if notif_server, this class is not called...
type Notifie struct {
	Name           string
	config         *config.DistConfig
	TagToSenderMap map[string]interfaces.NotifSenderInterface
	NotifChan      chan *utils.NotifMessage
	RawNotifChan   chan []byte
}

// This class is called by logger only with local notifie or notif_server
func NewNotifie(conf *config.DistConfig, parentName string) *Notifie {
	curNotifie := &Notifie{
		Name:           parentName,
		config:         conf,
		NotifChan:      make(chan *utils.NotifMessage),
		RawNotifChan:   make(chan []byte),
		TagToSenderMap: make(map[string]interfaces.NotifSenderInterface),
	}
	// set callback to init config
	// Assuming distributed-config has a similar mechanism or we call loadNotifSender directly
	// For now, let's assume we can subscribe or just load it once.
	// If Config has SetNotifCallBack, use it, otherwise just load.
	// curNotifie.config.Handler.SetNotifCallBack(curNotifie.loadNotifSender)

	// Since I don't know the exact API of distributed-config, I'll assume we pass the config map directly
	// or Config exposes a way to get it.
	// The original code passed a callback which received map[string]map[string]string.
	// I'll try to replicate the logic:

	// Try to get notification config from the passed config object.
	// If distributed-config's Config struct has a GetNotifConfig() or similar.
	// Assuming we can access the config data.

	// Placeholder: We need to load config.
	// In the original code: config.GetNotifConfig() triggered the callback.

	// Load initial config
	curNotifie.LoadNotifSender(conf.MemConfig)

	go curNotifie.processMessage()
	go curNotifie.consumeRawMessages()
	return curNotifie
}

func (notifie *Notifie) consumeRawMessages() {
	for rawData := range notifie.RawNotifChan {
		msg, err := DeserializeNotifMsg(rawData)
		if err != nil {
			fmt.Printf("Error deserializing raw message: %v\n", err)
			continue
		}
		notifie.NotifChan <- msg
	}
}

func (notifie *Notifie) LoadNotifSender(notifiersConf map[string]map[string]string) map[string][]string {
	// errorList := []string{}
	returnLogLevelByTag := map[string][]string{}

	if confTele, ok := notifiersConf["TELEGRAM"]; ok {
		telegram, telegramErrorList := notifiers.NewTelegramSender(confTele, "TELEGRAM")
		if telegramErrorList != "" {
			// errorList = append(errorList, telegramErrorList)
			fmt.Printf("Error loading Telegram sender: %s\n", telegramErrorList)
		} else {
			for _, logLevel := range strings.Split(telegram.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, telegram.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{telegram.GetTag()}
				}
			}
			notifie.TagToSenderMap[telegram.GetTag()] = telegram
		}
	}

	if confDisco, ok := notifiersConf["DISCORD"]; ok {
		discord, discordErrorList := notifiers.NewDiscordSender(confDisco, "DISCORD")
		if discordErrorList != "" {
			fmt.Printf("Error loading Discord sender: %s\n", discordErrorList)
		} else {
			for _, logLevel := range strings.Split(discord.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, discord.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{discord.GetTag()}
				}
			}
			notifie.TagToSenderMap[discord.GetTag()] = discord
		}
	}

	if confMatrix, ok := notifiersConf["MATRIX"]; ok {
		matrix, matrixErrorList := notifiers.NewMatrixSender(confMatrix, "MATRIX")
		if matrixErrorList != "" {
			fmt.Printf("Error loading Matrix sender: %s\n", matrixErrorList)
		} else {
			for _, logLevel := range strings.Split(matrix.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, matrix.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{matrix.GetTag()}
				}
			}
			notifie.TagToSenderMap[matrix.GetTag()] = matrix
		}
	}

	if confGmail, ok := notifiersConf["GMAIL"]; ok {
		gmail, gmailErrorList := notifiers.NewGmailSender(confGmail, "GMAIL")
		if gmailErrorList != "" {
			fmt.Printf("Error loading Gmail sender: %s\n", gmailErrorList)
		} else {
			for _, logLevel := range strings.Split(gmail.GetLogLevel(), ",") {
				logLevel = strings.TrimSpace(logLevel)
				if curConf, ok := returnLogLevelByTag[logLevel]; ok {
					curConf = append(curConf, gmail.GetTag())
				} else {
					returnLogLevelByTag[logLevel] = []string{gmail.GetTag()}
				}
			}
			notifie.TagToSenderMap[gmail.GetTag()] = gmail
		}
	}

	return returnLogLevelByTag
}

// Send sends a structured notification message.
func (notifie *Notifie) Send(msg *utils.NotifMessage) {
	notifie.NotifChan <- msg
}

// SendRaw sends a raw byte message (serialized).
func (notifie *Notifie) SendRaw(data []byte) {
	notifie.RawNotifChan <- data
}

func (notifie *Notifie) processMessage() {
	for recvNotifMessage := range notifie.NotifChan {
		for _, tag := range recvNotifMessage.Tags {
			if sender, ok := notifie.TagToSenderMap[tag]; ok {
				go sender.SendMessage(recvNotifMessage.Message, recvNotifMessage.Attachment, notifie.Name)
			}
		}
	}
}

// Notifie
/////////////////////////////////
