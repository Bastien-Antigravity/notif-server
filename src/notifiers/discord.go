package notifiers

/*
ESSENTIAL PROCESS:
Implements the Discord notification sender via Webhooks.
Executes HTTP POST requests to Discord with exponential backoff.
Dispatches operational and delivery messages through the ecosystem Universal Logger.

DATA FLOW:
1. NewDiscordSender parses and decrypts the webhook URL once at instantiation.
2. Marshals message into JSON with "content" field.
3. Posts to the Discord Webhook URL with retry backoff.
4. Logs dispatch lifecycle and delivery results through Logger.

KEY PARAMETERS:
- discordUrl: The decrypted direct Discord Webhook URL.
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

	"github.com/Bastien-Antigravity/notif-server/src/sanitizer"

	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type DiscordSender struct {
	tag        string
	discordUrl string
	logLevel   string
	logger     log_interfaces.Logger
	client     *http.Client
}

// -----------------------------------------------------------------------------

// NewDiscordSender initializes a Discord notification sender.
// Decrypts webhook URL once at instantiation. Returns (nil, nil) if unconfigured,
// or (nil, error) if decryption fails.
func NewDiscordSender(discordConf map[string]string, confName string, logger log_interfaces.Logger, decrypt func(string) (string, error)) (*DiscordSender, error) {
	tag := getOption(discordConf, "TAG", "tag")
	if tag == "" {
		tag = confName
	}

	discordUrl := getOption(discordConf, "URL", "url")
	if discordUrl == "" {
		logger.Warning("[%s] Discord provider configuration incomplete: missing required parameter 'URL'", tag)
		return nil, nil
	}

	// Decrypt webhook URL once at instantiation
	if decrypt != nil {
		dec, err := decrypt(discordUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt discord webhook url: %w", err)
		}
		discordUrl = dec
	}

	if !strings.HasPrefix(discordUrl, "http://") && !strings.HasPrefix(discordUrl, "https://") {
		logger.Warning("[%s] Discord provider configuration invalid: 'URL' must start with 'http://' or 'https://'", tag)
		return nil, nil
	}

	logLevel := getOption(discordConf, "LOGLEVEL", "loglevel")
	if logLevel == "" {
		logLevel = "INFO"
	}

	logger.Info("[%s] Discord notification provider initialized", tag)

	return &DiscordSender{
		tag:        tag,
		discordUrl: discordUrl,
		logLevel:   logLevel,
		logger:     logger,
		client:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// -----------------------------------------------------------------------------

// redactError strips the webhook URL and any other secrets from transport errors via the standard sanitizer.
func (discordSender *DiscordSender) redactError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if discordSender.discordUrl != "" {
		errStr = strings.ReplaceAll(errStr, discordSender.discordUrl, "[REDACTED_WEBHOOK_URL]")
	}
	return fmt.Errorf("%s", sanitizer.RedactSecrets(errStr))
}

// -----------------------------------------------------------------------------

func (discordSender *DiscordSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	discordSender.logger.Debug("[%s] Sending notification to Discord...", discordSender.tag)

	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		discordSender.logger.Error("[%s] Failed to marshal message (discord)", discordSender.tag)
		return fmt.Errorf("failed to marshal message (discord): %w", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", discordSender.discordUrl, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			safeErr := discordSender.redactError(err)
			discordSender.logger.Error("[%s] Failed to create HTTP request (discord): %v", discordSender.tag, safeErr)
			return fmt.Errorf("failed to create request (discord): %w", safeErr)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := discordSender.client.Do(req)
		if err == nil && httpsResp != nil {
			_, _ = io.Copy(io.Discard, httpsResp.Body)
			httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK || httpsResp.StatusCode == http.StatusNoContent {
				discordSender.logger.Info("[%s] Notification delivered successfully to Discord", discordSender.tag)
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (discord): %d", httpsResp.StatusCode)

			// Fatal errors (4xx but not 429) should not be retried
			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				discordSender.logger.Error("[%s] Non-retryable HTTP error from Discord: %d", discordSender.tag, httpsResp.StatusCode)
				return lastErr
			}
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = discordSender.redactError(fmt.Errorf("failed to post http request (discord): %w", err))
		}

		discordSender.logger.Warning("[%s] Discord delivery attempt %d failed: %v. Retrying in %v...", discordSender.tag, i+1, lastErr, backoff)

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	discordSender.logger.Error("[%s] Discord send failed after %d retries: %v", discordSender.tag, maxRetries, lastErr)
	return fmt.Errorf("discord send failed after %d retries: %w", maxRetries, lastErr)
}

// -----------------------------------------------------------------------------

func (d *DiscordSender) GetTag() string {
	return d.tag
}

// -----------------------------------------------------------------------------

func (d *DiscordSender) GetLogLevel() string {
	return d.logLevel
}
