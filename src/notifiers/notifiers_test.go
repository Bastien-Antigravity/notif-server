package notifiers

/*
ESSENTIAL PROCESS:
Unit tests for notification sender implementations (Matrix, Discord, Telegram, Gmail).
Validates optional configuration handling, seamless TAG fallback, and Logger integration.

DATA FLOW:
1. Provide various configurations (empty, partial, valid) to constructors.
2. Assert that missing or empty configs return (nil, nil) without failing.
3. Assert that valid configs instantiate properly with confName as default tag.
4. Execute mock HTTP delivery and assert response parsing and retry behavior.

KEY PARAMETERS:
- t: Testing context.
*/

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTestLogger captures logs for test assertion
type mockTestLogger struct {
	debugLogs []string
	infoLogs  []string
	warnLogs  []string
	errorLogs []string
}

func (m *mockTestLogger) Debug(format string, args ...any) {
	m.debugLogs = append(m.debugLogs, fmt.Sprintf(format, args...))
}
func (m *mockTestLogger) Info(format string, args ...any) {
	m.infoLogs = append(m.infoLogs, fmt.Sprintf(format, args...))
}
func (m *mockTestLogger) Warning(format string, args ...any) {
	m.warnLogs = append(m.warnLogs, fmt.Sprintf(format, args...))
}
func (m *mockTestLogger) Error(format string, args ...any) {
	m.errorLogs = append(m.errorLogs, fmt.Sprintf(format, args...))
}
func (m *mockTestLogger) Critical(format string, args ...any) {}
func (m *mockTestLogger) Logon(format string, args ...any)    {}
func (m *mockTestLogger) Logout(format string, args ...any)   {}
func (m *mockTestLogger) Trade(format string, args ...any)    {}
func (m *mockTestLogger) Schedule(format string, args ...any) {}
func (m *mockTestLogger) Report(format string, args ...any)   {}
func (m *mockTestLogger) Stream(format string, args ...any)   {}
func (m *mockTestLogger) SetLevel(level unilog_interfaces.Level) {}
func (m *mockTestLogger) GetLevel() unilog_interfaces.Level     { return unilog_interfaces.LevelDebug }
func (m *mockTestLogger) SetCallerSkip(skip int)                {}
func (m *mockTestLogger) SetMetadata(metadata map[string]string) {}
func (m *mockTestLogger) AddMetadata(key, value string)         {}
func (m *mockTestLogger) GetMetadata() map[string]string        { return nil }
func (m *mockTestLogger) Close()                                {}
func (m *mockTestLogger) Log(level unilog_interfaces.Level, format string, args ...any) {}
func (m *mockTestLogger) LogWithCaller(level unilog_interfaces.Level, msg, file, line, function, module string) {}
func (m *mockTestLogger) GetNotifQueue() <-chan *unilog_interfaces.NotifMessage { return nil }
func (m *mockTestLogger) SetLocalNotifQueue(notifChan chan *unilog_interfaces.NotifMessage) {}

// -----------------------------------------------------------------------------

func TestTelegramOptionalConfig(t *testing.T) {
	logger := &mockTestLogger{}

	// 1. Empty config should gracefully return nil without error
	sender, err := NewTelegramSender(map[string]string{}, "telegram_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)
	assert.NotEmpty(t, logger.debugLogs)

	// 2. Partial config (missing chatId) should gracefully return nil without error
	sender, err = NewTelegramSender(map[string]string{
		"TOKEN": "123456:ABC-DEF",
	}, "telegram_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 3. Empty fields should return nil
	sender, err = NewTelegramSender(map[string]string{
		"TOKEN":  "",
		"CHATID": "",
	}, "telegram_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 4. Valid config without explicit TAG uses confName
	sender, err = NewTelegramSender(map[string]string{
		"TOKEN":  "123456:VALID_TOKEN",
		"CHATID": "-100123456789",
	}, "telegram_alerts", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)
	assert.Equal(t, "telegram_alerts", sender.GetTag())
	assert.Equal(t, "INFO", sender.GetLogLevel())
}

// -----------------------------------------------------------------------------

func TestDiscordOptionalConfig(t *testing.T) {
	logger := &mockTestLogger{}

	// 1. Empty config should return nil without error
	sender, err := NewDiscordSender(map[string]string{}, "discord_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 2. Empty URL should return nil without error
	sender, err = NewDiscordSender(map[string]string{
		"URL": "",
	}, "discord_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 3. Valid config without explicit TAG uses confName
	sender, err = NewDiscordSender(map[string]string{
		"URL":      "https://discord.com/api/webhooks/123/abc",
		"LOGLEVEL": "CRITICAL",
	}, "discord_alerts", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)
	assert.Equal(t, "discord_alerts", sender.GetTag())
	assert.Equal(t, "CRITICAL", sender.GetLogLevel())
}

// -----------------------------------------------------------------------------

func TestMatrixOptionalConfig(t *testing.T) {
	logger := &mockTestLogger{}

	// 1. Empty config should return nil without error
	sender, err := NewMatrixSender(map[string]string{}, "matrix_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 2. Valid config without explicit TAG uses confName
	sender, err = NewMatrixSender(map[string]string{
		"URL": "https://matrix.example.com/_matrix/hook/123",
	}, "matrix_alerts", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)
	assert.Equal(t, "matrix_alerts", sender.GetTag())
	assert.Equal(t, "INFO", sender.GetLogLevel())
}

// -----------------------------------------------------------------------------

func TestGmailOptionalConfig(t *testing.T) {
	logger := &mockTestLogger{}

	// 1. Empty config should return nil without error
	sender, err := NewGmailSender(map[string]string{}, "gmail_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 2. Partial config should return nil without error
	sender, err = NewGmailSender(map[string]string{
		"FROM": "test@gmail.com",
		"TO":   "dest@gmail.com",
	}, "gmail_test", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sender)

	// 3. Valid config without explicit TAG uses confName and parses host/port
	sender, err = NewGmailSender(map[string]string{
		"FROM":      "sender@example.com",
		"TO":        "alerts@example.com",
		"PASSWD":    "secretpass",
		"SMTP_HOST": "mail.example.com",
		"SMTP_PORT": "465",
	}, "gmail_alerts", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)
	assert.Equal(t, "gmail_alerts", sender.GetTag())
	assert.Equal(t, "mail.example.com", sender.smtp)
	assert.Equal(t, 465, sender.port)
	assert.Equal(t, "CRITICAL", sender.GetLogLevel())
}

// -----------------------------------------------------------------------------

func TestExplicitParameterValidation(t *testing.T) {
	// 1. Telegram missing TOKEN
	logger := &mockTestLogger{}
	s, err := NewTelegramSender(map[string]string{"CHATID": "12345"}, "tg", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, s)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "'TOKEN'")

	// 2. Telegram invalid URL
	logger = &mockTestLogger{}
	s, err = NewTelegramSender(map[string]string{"TOKEN": "abc", "CHATID": "123", "URL": "ftp://bad"}, "tg", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, s)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "invalid: 'URL'")

	// 3. Discord missing URL
	logger = &mockTestLogger{}
	sDisc, err := NewDiscordSender(map[string]string{}, "discord", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sDisc)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "missing required parameter 'URL'")

	// 4. Discord invalid URL
	logger = &mockTestLogger{}
	sDisc, err = NewDiscordSender(map[string]string{"URL": "invalid-url"}, "discord", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sDisc)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "invalid: 'URL'")

	// 5. Matrix missing URL
	logger = &mockTestLogger{}
	sMat, err := NewMatrixSender(map[string]string{}, "matrix", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sMat)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "missing required parameter 'URL'")

	// 6. Matrix invalid URL
	logger = &mockTestLogger{}
	sMat, err = NewMatrixSender(map[string]string{"URL": "bad-matrix-url"}, "matrix", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sMat)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "invalid: 'URL'")

	// 7. Gmail missing PASSWD
	logger = &mockTestLogger{}
	sGmail, err := NewGmailSender(map[string]string{
		"FROM": "test@example.com",
		"TO":   "dest@example.com",
	}, "gmail", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sGmail)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "'PASSWD'")

	// 8. Gmail invalid FROM email
	logger = &mockTestLogger{}
	sGmail, err = NewGmailSender(map[string]string{
		"FROM":   "not-an-email",
		"TO":     "dest@example.com",
		"PASSWD": "pass",
	}, "gmail", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sGmail)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "invalid: 'FROM'")

	// 9. Gmail invalid SMTP_PORT
	logger = &mockTestLogger{}
	sGmail, err = NewGmailSender(map[string]string{
		"FROM":      "test@example.com",
		"TO":        "dest@example.com",
		"PASSWD":    "pass",
		"SMTP_PORT": "999999",
	}, "gmail", logger, nil)
	require.NoError(t, err)
	assert.Nil(t, sGmail)
	require.NotEmpty(t, logger.warnLogs)
	assert.Contains(t, logger.warnLogs[len(logger.warnLogs)-1], "invalid: 'SMTP_PORT'")
}

// -----------------------------------------------------------------------------

func TestMatrixSendMessageMock(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := &mockTestLogger{}
	sender, err := NewMatrixSender(map[string]string{
		"URL": server.URL,
	}, "matrix_mock", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sender.SendMessage(ctx, "Test Matrix Message", "", "")
	assert.NoError(t, err)
	assert.True(t, called)
	assert.NotEmpty(t, logger.infoLogs)
}

// -----------------------------------------------------------------------------

func TestDiscordSendMessageMock(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := &mockTestLogger{}
	sender, err := NewDiscordSender(map[string]string{
		"URL": server.URL,
	}, "discord_mock", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sender.SendMessage(ctx, "Test Discord Message", "", "")
	assert.NoError(t, err)
	assert.True(t, called)
	assert.NotEmpty(t, logger.infoLogs)
}

// -----------------------------------------------------------------------------

func TestTelegramSendMessageMock(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	logger := &mockTestLogger{}
	sender, err := NewTelegramSender(map[string]string{
		"TOKEN":  "mock_token",
		"CHATID": "12345",
		"URL":    server.URL,
	}, "telegram_mock", logger, nil)
	require.NoError(t, err)
	require.NotNil(t, sender)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sender.SendMessage(ctx, "Test Telegram Message", "", "")
	assert.NoError(t, err)
	assert.True(t, called)
	assert.NotEmpty(t, logger.infoLogs)
}

// -----------------------------------------------------------------------------

func TestTelegramOnDemandDecryption(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "/botdecrypted_token/sendMessage", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	logger := &mockTestLogger{}
	mockDecrypter := func(ciphertext string) (string, error) {
		if ciphertext == "ENC(mock_encrypted_token)" {
			return "decrypted_token", nil
		}
		if ciphertext == "ENC(mock_encrypted_chat)" {
			return "99999", nil
		}
		return ciphertext, nil
	}

	sender, err := NewTelegramSender(map[string]string{
		"TOKEN":  "ENC(mock_encrypted_token)",
		"CHATID": "ENC(mock_encrypted_chat)",
		"URL":    server.URL,
	}, "telegram_enc_test", logger, mockDecrypter)

	require.NoError(t, err)
	require.NotNil(t, sender)

	// Verify token remains stored encrypted in memory
	assert.Equal(t, "ENC(mock_encrypted_token)", sender.token)
	assert.Equal(t, "ENC(mock_encrypted_chat)", sender.chatId)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sender.SendMessage(ctx, "Encrypted Dispatch Test", "", "")
	assert.NoError(t, err)
	assert.True(t, called)

	// Assert that secrets were NOT logged
	for _, log := range logger.infoLogs {
		assert.NotContains(t, log, "decrypted_token")
		assert.NotContains(t, log, "99999")
	}
}

// -----------------------------------------------------------------------------

func TestDiscordOnDemandDecryption(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := &mockTestLogger{}
	mockDecrypter := func(ciphertext string) (string, error) {
		if ciphertext == "ENC(mock_webhook_secret)" {
			return server.URL, nil
		}
		return ciphertext, nil
	}

	sender, err := NewDiscordSender(map[string]string{
		"URL": "ENC(mock_webhook_secret)",
	}, "discord_enc_test", logger, mockDecrypter)

	require.NoError(t, err)
	require.NotNil(t, sender)

	// Verify raw URL remains stored as ciphertext
	assert.Equal(t, "ENC(mock_webhook_secret)", sender.discordUrl)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sender.SendMessage(ctx, "Discord Encrypted Test", "", "")
	assert.NoError(t, err)
	assert.True(t, called)

	// Assert no webhook URL or secrets in logs
	for _, log := range logger.infoLogs {
		assert.NotContains(t, log, server.URL)
	}
}
