package notifiers

/*
ESSENTIAL PROCESS:
Implements the Telegram notification sender.
Handles authentication with the Telegram Bot API and executes message delivery.
Dispatches operational and delivery messages through the ecosystem Universal Logger.

DATA FLOW:
1. Receives message payload from the core worker.
2. Marshals the message into JSON.
3. Posts to the Telegram sendMessage endpoint with exponential backoff.
4. Logs dispatch lifecycle and delivery results through Logger.

KEY PARAMETERS:
- apiURL: The full Telegram Bot API URL including token.
- chatId: Target Telegram chat or channel ID.
- logger: Unified logger instance for operational messages.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type TelegramSender struct {
	tag      string
	baseURL  string
	token    string
	chatId   string
	logLevel string
	logger   log_interfaces.Logger
	decrypt  func(string) (string, error)
}

// -----------------------------------------------------------------------------

// NewTelegramSender initializes a Telegram notification sender.
// Configuration is optional: if token or chatId are omitted or empty, the provider
// is considered unconfigured and returns (nil, nil) without failing.
func NewTelegramSender(telegramConf map[string]string, confName string, logger log_interfaces.Logger, decrypt func(string) (string, error)) (*TelegramSender, error) {
	tag := getOption(telegramConf, "TAG", "tag")
	if tag == "" {
		tag = confName
	}

	token := getOption(telegramConf, "TOKEN", "token")
	chatId := getOption(telegramConf, "CHATID", "chatid", "chat_id")

	var missing []string
	if token == "" {
		missing = append(missing, "'TOKEN'")
	}
	if chatId == "" {
		missing = append(missing, "'CHATID'")
	}
	if len(missing) > 0 {
		logger.Warning("[%s] Telegram provider configuration incomplete: missing required parameter(s) %s", tag, strings.Join(missing, ", "))
		logger.Debug("[%s] Telegram provider not configured: missing %s; skipping", tag, strings.Join(missing, ", "))
		return nil, nil
	}

	baseURL := getOption(telegramConf, "URL", "url")
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		logger.Warning("[%s] Telegram provider configuration invalid: 'URL' must start with 'http://' or 'https://'", tag)
		return nil, nil
	}
	baseURL = strings.TrimRight(baseURL, "/")

	logLevel := getOption(telegramConf, "LOGLEVEL", "loglevel")
	if logLevel == "" {
		logLevel = "INFO"
	}

	logger.Info("[%s] Telegram notification provider initialized", tag)

	return &TelegramSender{
		tag:      tag,
		baseURL:  baseURL,
		token:    token,
		chatId:   chatId,
		logLevel: logLevel,
		logger:   logger,
		decrypt:  decrypt,
	}, nil
}

// -----------------------------------------------------------------------------

func (ts *TelegramSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	ts.logger.Debug("[%s] Sending notification to Telegram...", ts.tag)

	// Decrypt credentials on-demand only when executing delivery
	plainToken, err := ts.decrypt(ts.token)
	if err != nil {
		ts.logger.Error("[%s] Failed to decrypt Telegram bot token", ts.tag)
		return fmt.Errorf("failed to decrypt telegram bot token: %w", err)
	}
	plainChatId, err := ts.decrypt(ts.chatId)
	if err != nil {
		ts.logger.Error("[%s] Failed to decrypt Telegram chat ID", ts.tag)
		return fmt.Errorf("failed to decrypt telegram chat id: %w", err)
	}

	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", ts.baseURL, plainToken)

	payload := map[string]string{
		"chat_id": plainChatId,
		"text":    msg,
	}
	jsonByteMessage, err := json.Marshal(payload)
	if err != nil {
		ts.logger.Error("[%s] Failed to marshal message (telegram): %v", ts.tag, err)
		return fmt.Errorf("failed to marshal message (telegram): %w", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			ts.logger.Error("[%s] Failed to create HTTP request (telegram): %v", ts.tag, err)
			return fmt.Errorf("failed to create request (telegram): %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := http.DefaultClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, httpsResp.Body)
			httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK {
				ts.logger.Info("[%s] Notification delivered successfully to Telegram", ts.tag)
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (telegram): %d", httpsResp.StatusCode)

			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				ts.logger.Error("[%s] Non-retryable HTTP error from Telegram: %d", ts.tag, httpsResp.StatusCode)
				return lastErr
			}
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("failed to post http request (telegram): %v", err)
		}

		ts.logger.Warning("[%s] Telegram delivery attempt %d failed: %v. Retrying in %v...", ts.tag, i+1, lastErr, backoff)

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	ts.logger.Error("[%s] Telegram send failed after %d retries: %v", ts.tag, maxRetries, lastErr)
	return fmt.Errorf("telegram send failed after %d retries: %w", maxRetries, lastErr)
}

// -----------------------------------------------------------------------------

func (ts *TelegramSender) GetTag() string {
	return ts.tag
}

// -----------------------------------------------------------------------------

func (ts *TelegramSender) GetLogLevel() string {
	return ts.logLevel
}
