package notifiers

/*
ESSENTIAL PROCESS:
Implements the Matrix notification sender.
Executes HTTP POST requests to Matrix webhook endpoints with exponential backoff.
Dispatches operational and delivery messages through the ecosystem Universal Logger.

DATA FLOW:
1. Receives message payload from the core worker.
2. Marshals the message into JSON with "content" field.
3. Posts to the Matrix integration URL with retry backoff.
4. Logs dispatch lifecycle and delivery results through Logger.

KEY PARAMETERS:
- matrixUrl: The full Matrix webhook or integration URL.
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

type MatrixSender struct {
	tag       string
	matrixUrl string
	logLevel  string
	logger    log_interfaces.Logger
	decrypt   func(string) (string, error)
}

// -----------------------------------------------------------------------------

// NewMatrixSender initializes a Matrix notification sender.
// Configuration is optional: if URL is omitted or empty, the provider is considered
// unconfigured and returns (nil, nil) without failing.
func NewMatrixSender(matrixConf map[string]string, confName string, logger log_interfaces.Logger, decrypt func(string) (string, error)) (*MatrixSender, error) {
	tag := getOption(matrixConf, "TAG", "tag")
	if tag == "" {
		tag = confName
	}

	matrixUrl := getOption(matrixConf, "URL", "url")
	if matrixUrl == "" {
		logger.Info("[%s] Matrix provider configuration incomplete: missing required parameter 'URL'", tag)
		return nil, nil
	}

	if !strings.HasPrefix(matrixUrl, "ENC(") && !strings.HasPrefix(matrixUrl, "http://") && !strings.HasPrefix(matrixUrl, "https://") {
		logger.Warning("[%s] Matrix provider configuration invalid: 'URL' must start with 'http://' or 'https://'", tag)
		return nil, nil
	}

	logLevel := getOption(matrixConf, "LOGLEVEL", "loglevel")
	if logLevel == "" {
		logLevel = "INFO"
	}

	logger.Info("[%s] Matrix notification provider initialized", tag)

	return &MatrixSender{
		tag:       tag,
		matrixUrl: matrixUrl,
		logLevel:  logLevel,
		logger:    logger,
		decrypt:   decrypt,
	}, nil
}

// -----------------------------------------------------------------------------

func (matrixSender *MatrixSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	matrixSender.logger.Debug("[%s] Sending notification to Matrix...", matrixSender.tag)

	// Decrypt webhook URL on-demand only when executing delivery
	plainURL, err := matrixSender.decrypt(matrixSender.matrixUrl)
	if err != nil {
		matrixSender.logger.Error("[%s] Failed to decrypt Matrix webhook URL", matrixSender.tag)
		return fmt.Errorf("failed to decrypt matrix webhook url: %w", err)
	}

	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		matrixSender.logger.Error("[%s] Failed to marshal message (matrix): %v", matrixSender.tag, err)
		return fmt.Errorf("failed to marshal message (matrix): %w", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error
	client := &http.Client{}

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", matrixSender.matrixUrl, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			matrixSender.logger.Error("[%s] Failed to create HTTP request (matrix): %v", matrixSender.tag, err)
			return fmt.Errorf("failed to create request (matrix): %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, httpsResp.Body)
			httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK {
				matrixSender.logger.Info("[%s] Notification delivered successfully to Matrix", matrixSender.tag)
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (matrix): %d", httpsResp.StatusCode)

			// Fatal errors (4xx but not 429) should not be retried
			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				if matrixSender.logger != nil {
					matrixSender.logger.Error("[%s] Non-retryable HTTP error from Matrix: %d", matrixSender.tag, httpsResp.StatusCode)
				}
				return lastErr
			}
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("failed to post http request (matrix): %v", err)
		}

		if matrixSender.logger != nil {
			matrixSender.logger.Warning("[%s] Matrix delivery attempt %d failed: %v. Retrying in %v...", matrixSender.tag, i+1, lastErr, backoff)
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

	if matrixSender.logger != nil {
		matrixSender.logger.Error("[%s] Matrix send failed after %d retries: %v", matrixSender.tag, maxRetries, lastErr)
	}
	return fmt.Errorf("matrix send failed after %d retries: %w", maxRetries, lastErr)
}

// -----------------------------------------------------------------------------

func (m *MatrixSender) GetTag() string {
	return m.tag
}

// -----------------------------------------------------------------------------

func (m *MatrixSender) GetLogLevel() string {
	return m.logLevel
}
