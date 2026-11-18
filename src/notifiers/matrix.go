package notifiers

/*
ESSENTIAL PROCESS:
Implements the Matrix notification sender.
Executes HTTP POST requests to Matrix webhook endpoints with exponential backoff.
Dispatches operational and delivery messages through the ecosystem Universal Logger.

DATA FLOW:
1. NewMatrixSender parses and decrypts the webhook URL once at instantiation.
2. Marshals message into JSON with "content" field.
3. Posts to the Matrix integration URL with retry backoff.
4. Logs dispatch lifecycle and delivery results through Logger.

KEY PARAMETERS:
- matrixUrl: The decrypted full Matrix webhook or integration URL.
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

type MatrixSender struct {
	tag       string
	matrixUrl string
	logLevel  string
	logger    log_interfaces.Logger
	client    *http.Client
}

// -----------------------------------------------------------------------------

// NewMatrixSender initializes a Matrix notification sender.
// Decrypts webhook URL once at instantiation. Returns (nil, nil) if unconfigured,
// or (nil, error) if decryption fails.
func NewMatrixSender(matrixConf map[string]string, confName string, logger log_interfaces.Logger, decrypt func(string) (string, error)) (*MatrixSender, error) {
	if logger == nil {
		return nil, fmt.Errorf("NewMatrixSender: logger must not be nil")
	}

	tag := getOption(matrixConf, "TAG", "tag")
	if tag == "" {
		tag = confName
	}

	matrixUrl := getOption(matrixConf, "URL", "url")
	if matrixUrl == "" {
		logger.Warning("[%s] Matrix provider configuration incomplete: missing required parameter 'URL'", tag)
		return nil, nil
	}

	// Decrypt webhook URL once at instantiation
	if decrypt != nil {
		dec, err := decrypt(matrixUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt matrix webhook url: %w", err)
		}
		matrixUrl = dec
	}

	if !strings.HasPrefix(matrixUrl, "http://") && !strings.HasPrefix(matrixUrl, "https://") {
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
		client:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// -----------------------------------------------------------------------------

// redactError strips the webhook URL and any other secrets from transport errors via the standard sanitizer.
func (matrixSender *MatrixSender) redactError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if matrixSender.matrixUrl != "" {
		errStr = strings.ReplaceAll(errStr, matrixSender.matrixUrl, "[REDACTED_WEBHOOK_URL]")
	}
	return fmt.Errorf("%s", sanitizer.RedactSecrets(errStr))
}

// -----------------------------------------------------------------------------

func (matrixSender *MatrixSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	matrixSender.logger.Debug("[%s] Sending notification to Matrix...", matrixSender.tag)

	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		matrixSender.logger.Error("[%s] Failed to marshal message (matrix): %v", matrixSender.tag, err)
		return fmt.Errorf("failed to marshal message (matrix): %w", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", matrixSender.matrixUrl, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			safeErr := matrixSender.redactError(err)
			matrixSender.logger.Error("[%s] Failed to create HTTP request (matrix): %v", matrixSender.tag, safeErr)
			return fmt.Errorf("failed to create request (matrix): %w", safeErr)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := matrixSender.client.Do(req)
		if err == nil && httpsResp != nil {
			_, _ = io.Copy(io.Discard, httpsResp.Body)
			httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK {
				matrixSender.logger.Info("[%s] Notification delivered successfully to Matrix", matrixSender.tag)
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (matrix): %d", httpsResp.StatusCode)

			// Fatal errors (4xx but not 429) should not be retried
			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				matrixSender.logger.Error("[%s] Non-retryable HTTP error from Matrix: %d", matrixSender.tag, httpsResp.StatusCode)
				return lastErr
			}
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = matrixSender.redactError(fmt.Errorf("failed to post http request (matrix): %w", err))
		}

		matrixSender.logger.Warning("[%s] Matrix delivery attempt %d failed: %v. Retrying in %v...", matrixSender.tag, i+1, lastErr, backoff)

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	matrixSender.logger.Error("[%s] Matrix send failed after %d retries: %v", matrixSender.tag, maxRetries, lastErr)
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
