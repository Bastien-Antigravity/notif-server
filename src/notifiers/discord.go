package notifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type DiscordSender struct {
	tag string
	// only used for discord
	discordUrl string // discord://WebhookID/WebhookToken/ direct url from the discord Room
	// Level
	logLevel string
}

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

func (discordSender *DiscordSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		return fmt.Errorf("failed to marshall message (discord): %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", discordSender.discordUrl, bytes.NewBuffer(jsonByteMessage))
	if err != nil {
		return fmt.Errorf("failed to create request (discord): %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	httpsResp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post http request (discord): %v", err)
	}
	defer httpsResp.Body.Close()

	if httpsResp.StatusCode != http.StatusOK && httpsResp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected http status (discord): %d", httpsResp.StatusCode)
	}
	return nil
}

func (d *DiscordSender) GetTag() string {
	return d.tag
}

func (d *DiscordSender) GetLogLevel() string {
	return d.logLevel
}
