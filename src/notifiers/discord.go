package notifiers

/*
ESSENTIAL PROCESS:
Implements the Discord notification sender via Webhooks.
Executes HTTP POST requests to Discord with exponential backoff.

DATA FLOW:
1. Receives message payload from the core worker.
2. Marshals the message into JSON with "content" field.
3. Posts to the Discord Webhook URL.

KEY PARAMETERS:
- discordUrl: The direct Discord Webhook URL.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DiscordSender struct {
	tag string
	// only used for discord
	discordUrl string // discord://WebhookID/WebhookToken/ direct url from the discord Room
	// Level
	logLevel string
}

// -----------------------------------------------------------------------------

func NewDiscordSender(discordConf map[string]string, confName string) (*DiscordSender, string) {
	curError := ""
	discordSender := &DiscordSender{}
	if tag, ok := discordConf["TAG"]; ok {
		discordSender.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}
	if discordUrl, ok := discordConf["URL"]; ok {
		discordSender.discordUrl = discordUrl
	} else {
		curError += fmt.Sprintf("missing 'URL' option for config '%s'\n", confName)
	}
	if logLevel, ok := discordConf["LOGLEVEL"]; ok {
		discordSender.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}
	if curError == "" {
		return discordSender, ""
	}
	return nil, curError
}

// -----------------------------------------------------------------------------

func (discordSender *DiscordSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		return fmt.Errorf("failed to marshall message (discord): %v", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error
	client := &http.Client{}

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", discordSender.discordUrl, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			return fmt.Errorf("failed to create request (discord): %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := client.Do(req)
		if err == nil {
			defer httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK || httpsResp.StatusCode == http.StatusNoContent {
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (discord): %d", httpsResp.StatusCode)

			// Fatal errors (4xx but not 429) should not be retried
			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				return lastErr
			}
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("failed to post http request (discord): %v", err)
		}

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return fmt.Errorf("discord send failed after %d retries: %v", maxRetries, lastErr)
}

// -----------------------------------------------------------------------------

func (d *DiscordSender) GetTag() string {
	return d.tag
}

// -----------------------------------------------------------------------------

func (d *DiscordSender) GetLogLevel() string {
	return d.logLevel
}
