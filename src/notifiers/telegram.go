package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TelegramSender struct {
	tag      string
	apiURL   string
	token    string
	chatId   string
	logLevel string
}

func NewTelegramSender(telegramConf map[string]string, confName string) (*TelegramSender, string) {
	curError := ""
	ts := &TelegramSender{}
	if tag, ok := telegramConf["TAG"]; ok {
		ts.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}
	if token, ok := telegramConf["TOKEN"]; ok {
		ts.token = token
	} else {
		curError += fmt.Sprintf("missing 'TOKEN' option for config '%s'\n", confName)
	}
	if chatId, ok := telegramConf["CHATID"]; ok {
		ts.chatId = chatId
	} else {
		curError += fmt.Sprintf("missing 'CHATID' option for config '%s'\n", confName)
	}
	if logLevel, ok := telegramConf["LOGLEVEL"]; ok {
		ts.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}

	if curError == "" {
		// Fix: Telegram API uses https, not tgram:// scheme.
		ts.apiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ts.token)
		return ts, ""
	}
	return nil, curError
}

func (ts *TelegramSender) SendMessage(msg, notUsed, notUsedAlso string) error {
	payload := map[string]string{
		"chat_id": ts.chatId,
		"text":    msg,
	}
	jsonByteMessage, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message (telegram): %v", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		httpsResp, err := http.Post(ts.apiURL, "application/json", bytes.NewBuffer(jsonByteMessage))
		if err == nil {
			defer httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (telegram): %d", httpsResp.StatusCode)
			
			// If it's a 4xx error (other than 429), don't retry as it's likely a client error (wrong token/chatId)
			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				return lastErr
			}
		} else {
			lastErr = fmt.Errorf("failed to post http request (telegram): %v", err)
		}

		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return fmt.Errorf("telegram send failed after %d retries: %v", maxRetries, lastErr)
}

func (ts *TelegramSender) GetTag() string {
	return ts.tag
}

func (ts *TelegramSender) GetLogLevel() string {
	return ts.logLevel
}
