package notifiers

/*
ESSENTIAL PROCESS:
Implements the Matrix notification sender.
Executes HTTP POST requests to Matrix webhook endpoints with exponential backoff.

DATA FLOW:
1. Receives message payload from the core worker.
2. Marshals the message into JSON with "content" field.
3. Posts to the Matrix integration URL.

KEY PARAMETERS:
- matrixUrl: The full Matrix webhook or integration URL.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type MatrixSender struct {
	tag string
	// only used for matrix
	matrixUrl string
	// Level
	logLevel string
}

// -----------------------------------------------------------------------------

func NewMatrixSender(matrixConf map[string]string, confName string) (*MatrixSender, string) {
	curError := ""
	matrixSender := &MatrixSender{}
	if tag, ok := matrixConf["TAG"]; ok {
		matrixSender.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}
	if matrixUrl, ok := matrixConf["URL"]; ok {
		matrixSender.matrixUrl = matrixUrl
	} else {
		curError += fmt.Sprintf("missing 'URL' option for config '%s'\n", confName)
	}
	if logLevel, ok := matrixConf["LOGLEVEL"]; ok {
		matrixSender.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}
	if curError == "" {
		return matrixSender, ""
	}
	return nil, curError
}

// -----------------------------------------------------------------------------

func (matrixSender *MatrixSender) SendMessage(ctx context.Context, msg, notUsed, notUsedAlso string) error {
	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		return fmt.Errorf("failed to marshall message (matrix): %v", err)
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	var lastErr error
	client := &http.Client{}

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", matrixSender.matrixUrl, bytes.NewBuffer(jsonByteMessage))
		if err != nil {
			return fmt.Errorf("failed to create request (matrix): %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		httpsResp, err := client.Do(req)
		if err == nil {
			defer httpsResp.Body.Close()
			if httpsResp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("unexpected http status (matrix): %d", httpsResp.StatusCode)

			// Fatal errors (4xx but not 429) should not be retried
			if httpsResp.StatusCode >= 400 && httpsResp.StatusCode < 500 && httpsResp.StatusCode != 429 {
				return lastErr
			}
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("failed to post http request (matrix): %v", err)
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

	return fmt.Errorf("matrix send failed after %d retries: %v", maxRetries, lastErr)
}

// -----------------------------------------------------------------------------

func (m *MatrixSender) GetTag() string {
	return m.tag
}

// -----------------------------------------------------------------------------

func (m *MatrixSender) GetLogLevel() string {
	return m.logLevel
}
