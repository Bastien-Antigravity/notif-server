package notifiers

/*
ESSENTIAL PROCESS:
Implements the Discord notification sender via Webhooks.
Executes HTTP POST requests to Discord with exponential backoff.
Dispatches operational and delivery messages through the ecosystem Universal Logger.

DATA FLOW:
1. Receives message payload from the core worker.
2. Marshals the message into JSON with "content" field.
3. Posts to the Discord Webhook URL with retry backoff.
4. Logs dispatch lifecycle and delivery results through Logger.

KEY PARAMETERS:
- discordUrl: The direct Discord Webhook URL.
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

type DiscordSender struct {
	tag        string
	discordUrl string
	logLevel   string
	logger     log_interfaces.Logger
	decrypt    func(string) (string, error)
}

// -----------------------------------------------------------------------------

// NewDiscordSender initializes a Discord notification sender.
// Configuration is optional: if URL is omitted or empty, the provider is considered
// unconfigured and returns (nil, nil) without failing.
func NewDiscordSender(discordConf map[string]string, confName string, logger log_interfaces.Logger, decrypt func(string) (string, error)) (*DiscordSender, error) {
	logger = EnsureSafeLogger(logger)

	tag := getOption(discordConf, "TAG", "tag")
	if tag == "" {
		tag = confName
	}

	discordUrl := getOption(discordConf, "URL", "url")
	if discordUrl == "" {
		logger.Info("[%s] Discord provider configuration incomplete: missing required parameter 'URL'", tag)
		return nil, nil
	}

	if !strings.HasPrefix(discordUrl, "ENC(") && !strings.HasPrefix(discordUrl, "http://") && !strings.HasPrefix(discordUrl, "https://") {
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
		decrypt:    decrypt,
	}, nil
}

// -----------------------------------------------------------------------------

func (discordSender *DiscordSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	discordSender.logger.Debug("[%s] Sending notification to Discord...", discordSender.tag)

	// Decrypt webhook URL on-demand only when executing delivery
	plainURL, err := discordSender.decrypt(discordSender.discordUrl)
	if err != nil {
		discordSender.logger.Error("[%s] Failed to decrypt Discord webhook URL", discordSender.tag)
		return fmt.Errorf("failed to decrypt discord webhook url: %w", err)
	}

	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		discordSender.logger.Error("[%s] Failed to marshal message (discord)", discordSender.tag)
		return fmt.Errorf("failed to marshal message (discord): %w", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error
	client := &http.Client{}

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", plainURL, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			discordSender.logger.Error("[%s] Failed to create HTTP request (discord): %v", discordSender.tag, err)
			return fmt.Errorf("failed to create request (discord): %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := client.Do(req)
		if err == nil {
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
			lastErr = fmt.Errorf("failed to post http request (discord): %v", err)
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
