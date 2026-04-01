package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type TelegramSender struct {
	tag string
	// only used for telegram
	telegramUrl string // -> tgram://{TOKEN}/{CHAT_ID}
	token       string
	chatId      string
	// Level
	logLevel string
}

func NewTelegramSender(telegramConf map[string]string, confName string) (*TelegramSender, string) {
	curError := ""
	telegranSender := &TelegramSender{}
	if tag, ok := telegramConf["TAG"]; ok {
		telegranSender.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}
	if token, ok := telegramConf["TOKEN"]; ok {
		telegranSender.token = token
	} else {
		curError += fmt.Sprintf("missing 'TOKEN' option for config '%s'\n", confName)
	}
	if chatId, ok := telegramConf["CHATID"]; ok {
		telegranSender.chatId = chatId
	} else {
		curError += fmt.Sprintf("missing 'CHATID' option for config '%s'\n", confName)
	}
	if logLevel, ok := telegramConf["LOGLEVEL"]; ok {
		telegranSender.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}
	if curError == "" {
		telegranSender.telegramUrl = fmt.Sprintf("tgram://%s/%s", telegranSender.token, telegranSender.chatId)
		return telegranSender, ""
	}
	return nil, curError
}

func (telegramSender *TelegramSender) SendMessage(msg, notUsed, notUsedAlso string) error {
	jsonByteMessage, err := json.Marshal(map[string]string{"chat_id": telegramSender.chatId, "text": msg})
	if err != nil {
		return fmt.Errorf("failed to marshall message (telegram): %v", err)
	}
	httpsResp, err := http.Post(telegramSender.telegramUrl, "application/json", bytes.NewBuffer(jsonByteMessage))
	if err != nil {
		return fmt.Errorf("failed to post http request (telegram): %v", err)
	}
	defer httpsResp.Body.Close()
	if httpsResp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram, unexpected http status (telegram): %d", httpsResp.StatusCode)
	}
	return nil
}

func (t *TelegramSender) GetTag() string {
	return t.tag
}

func (t *TelegramSender) GetLogLevel() string {
	return t.logLevel
}
