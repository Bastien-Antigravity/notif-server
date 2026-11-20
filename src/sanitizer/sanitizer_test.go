package sanitizer

/*
ESSENTIAL PROCESS:
Unit tests verifying the deny-by-default config exposure allowlist, header/filename sanitization,
and exhaustive secret redaction across URLs, tokens, headers, and ENC(...) blobs.

DATA FLOW:
1. Input: Synthetic public/secret config fields, malicious CRLF/NUL strings, and error strings containing secrets.
2. Logic: Invokes IsPublicField, SanitizeHeader, SanitizeFilename, and RedactSecrets.
3. Output: Asserts zero secret leakage and complete control-character removal.

KEY PARAMETERS:
- t: Testing context.
*/

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------

func TestIsPublicFieldDenyByDefault(t *testing.T) {
	// Explicitly allowed public fields
	assert.True(t, IsPublicField("TELEGRAM", "URL", "https://api.telegram.org"))
	assert.True(t, IsPublicField("TELEGRAM", "CHATID", "123456789"))
	assert.True(t, IsPublicField("GMAIL", "FROM", "alerts@example.com"))
	assert.True(t, IsPublicField("GMAIL", "ATTACHMENT_DIR", "/tmp/notif-attachments"))
	assert.True(t, IsPublicField("DISCORD", "LOGLEVEL", "ERROR"))

	// Secrets & unknown fields are denied by default
	assert.False(t, IsPublicField("TELEGRAM", "TOKEN", "123:abc"))
	assert.False(t, IsPublicField("DISCORD", "URL", "https://discord.com/api/webhooks/1/secret"))
	assert.False(t, IsPublicField("MATRIX", "URL", "https://matrix.local/hook"))
	assert.False(t, IsPublicField("GMAIL", "PASSWD", "app-pass"))
	assert.False(t, IsPublicField("DISCORD", "FUTURE_CUSTOM_FIELD", "anything"))
	assert.False(t, IsPublicField("UNKNOWN_DRIVER", "API_CRED", "secret"))

	// Even normally-public fields are denied if they carry ENC(...) or embedded secrets
	assert.False(t, IsPublicField("TELEGRAM", "CHATID", "ENC(cipher)"))
	assert.False(t, IsPublicField("TELEGRAM", "URL", "https://user:pass@proxy.local"))
	assert.False(t, IsPublicField("GMAIL", "FROM", "https://discord.com/api/webhooks/1/tok"))
}

// -----------------------------------------------------------------------------

func TestSanitizeHeaderAndFilename(t *testing.T) {
	assert.Equal(t, "AlertBcc: evil@x.com", SanitizeHeader("Alert\r\nBcc: evil@x.com\x00\u0085\u2028"))
	assert.Equal(t, "report.pdf", SanitizeFilename("/tmp/dir/report.pdf"))
	assert.Equal(t, "badname.txt", SanitizeFilename("bad\"na;m'e\r\n.txt"))
	assert.Equal(t, "attachment", SanitizeFilename(".../../"))
}

// -----------------------------------------------------------------------------

func TestRedactSecretsExhaustive(t *testing.T) {
	raw := `failed calling https://discord.com/api/webhooks/99/SECRET_HOOK and https://user:pw123@host/bot12345:AAbbCC_dd/sendMessage?access_token=TOK999 with Bearer eyJhbGciOi.payload Basic dXNlcjpwYXNzMTIz and {"password":"JSON_SECRET_PW"} and ENC(abcd==) and MY_LITERAL_SECRET`
	clean := RedactSecrets(raw, "MY_LITERAL_SECRET")

	for _, leaked := range []string{
		"SECRET_HOOK",
		"pw123",
		"AAbbCC_dd",
		"TOK999",
		"eyJhbGciOi.payload",
		"dXNlcjpwYXNzMTIz",
		"JSON_SECRET_PW",
		"ENC(abcd==)",
		"MY_LITERAL_SECRET",
	} {
		assert.NotContains(t, clean, leaked)
	}
	assert.Contains(t, clean, "[REDACTED_WEBHOOK_URL]")
	assert.Contains(t, clean, "[REDACTED_BOT_TOKEN]")
	assert.Contains(t, clean, "[REDACTED_ENC]")
	assert.Contains(t, clean, "[REDACTED]")
}
