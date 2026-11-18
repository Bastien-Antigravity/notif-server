package sanitizer

/*
ESSENTIAL PROCESS:
Provides the standard, exhaustive security sanitization and secret-redaction library for notif-server.
Enforces deny-by-default config exposure for web-interface, strips injection characters from headers
and filenames, and scrubs credentials from logs and error telemetry.

DATA FLOW:
1. Input: Raw config key/values, email headers, attachment filenames, or transport error strings.
2. Logic: Applies explicit allowlist checks, control-character stripping, and regex/literal secret scrubbing.
3. Output: Safe strings guaranteed free of credential leaks or MIME/CRLF injection vectors.

KEY PARAMETERS:
- MaskedPlaceholder: Canonical masked string ("********") returned to UIs for write-only secrets.
- publicConfigFields: Explicit per-provider allowlist of non-sensitive configuration keys.
*/

import (
	"path/filepath"
	"regexp"
	"strings"
)

// MaskedPlaceholder is the canonical placeholder returned to UIs for sensitive fields.
const MaskedPlaceholder = "********"

// -----------------------------------------------------------------------------

// publicConfigFields defines the ONLY non-sensitive keys safe to expose to web-interface per provider type.
// Any key not explicitly listed here (including future/unknown keys or ENC(...) values) is masked by default.
var publicConfigFields = map[string]map[string]bool{
	"TELEGRAM": {
		"TYPE":     true,
		"TAG":      true,
		"LOGLEVEL": true,
		"URL":      true,
		"CHATID":   true,
		"CHAT_ID":  true,
	},
	"DISCORD": {
		"TYPE":     true,
		"TAG":      true,
		"LOGLEVEL": true,
	},
	"MATRIX": {
		"TYPE":     true,
		"TAG":      true,
		"LOGLEVEL": true,
	},
	"GMAIL": {
		"TYPE":           true,
		"TAG":            true,
		"LOGLEVEL":       true,
		"FROM":           true,
		"TO":             true,
		"SMTP_HOST":      true,
		"SMTP_PORT":      true,
		"HOST":           true,
		"PORT":           true,
		"ATTACHMENT_DIR": true,
	},
}

var (
	reTelegramBotURL  = regexp.MustCompile(`/bot[0-9]+:[A-Za-z0-9_-]+`)
	reDiscordWebhook  = regexp.MustCompile(`https?://[^/\s"'<>]*discord(?:app)?\.com/api/webhooks/[^\s"'<>]+`)
	reURLBasicAuth    = regexp.MustCompile(`(https?://)([^:@/\s]+):([^@/\s]+)@`)
	reQuerySecret     = regexp.MustCompile(`(?i)([?&](?:access_token|token|api_key|apikey|key|secret|password|passwd|auth)=)([^&\s"'<>]+)`)
	reBearerToken     = regexp.MustCompile(`(?i)(Bearer\s+)([A-Za-z0-9._~+/=-]+)`)
	reBasicAuthHeader = regexp.MustCompile(`(?i)(Basic\s+)([A-Za-z0-9+/=]{8,})`)
	reJSONSecretField = regexp.MustCompile(`(?i)("(?:password|passwd|token|secret|api_key|apikey|access_token|webhook_url)"\s*:\s*")([^"]+)(")`)
	reEncryptedBlob   = regexp.MustCompile(`ENC\([^)]*\)`)
)

// -----------------------------------------------------------------------------

// IsPublicField reports whether a provider config field is explicitly safe to return to web-interface.
// Uses a deny-by-default allowlist: any unknown field, ENC(...) value, or embedded secret URL returns false.
func IsPublicField(platformType, key, value string) bool {
	trimmedVal := strings.TrimSpace(value)
	if strings.HasPrefix(trimmedVal, "ENC(") ||
		strings.Contains(trimmedVal, "/api/webhooks/") ||
		reTelegramBotURL.MatchString(trimmedVal) ||
		reURLBasicAuth.MatchString(trimmedVal) {
		return false
	}

	upperType := strings.ToUpper(strings.TrimSpace(platformType))
	upperKey := strings.ToUpper(strings.TrimSpace(key))

	allowed, ok := publicConfigFields[upperType]
	if !ok {
		// Unknown provider type: only allow generic metadata keys
		return upperKey == "TYPE" || upperKey == "TAG" || upperKey == "LOGLEVEL"
	}
	return allowed[upperKey]
}

// -----------------------------------------------------------------------------

// SanitizeHeader strips all ASCII/C1 control characters (0x00-0x1F, 0x7F-0x9F including CR/LF/NUL/NEL)
// and Unicode line/paragraph separators to prevent MIME/HTTP header injection.
func SanitizeHeader(val string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == '\u2028' || r == '\u2029' {
			return -1
		}
		return r
	}, val)
}

// -----------------------------------------------------------------------------

// SanitizeFilename returns a safe base filename stripped of control characters,
// quotes, backslashes, and MIME parameter delimiters.
func SanitizeFilename(name string) string {
	base := filepath.Base(name)
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == '"' || r == '\'' || r == '\\' || r == ';' || r == '/' {
			return -1
		}
		return r
	}, base)
	if clean == "" || clean == "." || clean == ".." {
		return "attachment"
	}
	return clean
}

// -----------------------------------------------------------------------------

// RedactSecrets scrubs both explicit known secret literals and common secret URL/token/JSON patterns
// from error strings and telemetry messages.
func RedactSecrets(input string, knownSecrets ...string) string {
	out := input
	for _, secret := range knownSecrets {
		if secret != "" && strings.Contains(out, secret) {
			out = strings.ReplaceAll(out, secret, "[REDACTED]")
		}
	}
	out = reTelegramBotURL.ReplaceAllString(out, "/bot[REDACTED_BOT_TOKEN]")
	out = reDiscordWebhook.ReplaceAllString(out, "[REDACTED_WEBHOOK_URL]")
	out = reURLBasicAuth.ReplaceAllString(out, "${1}[REDACTED]@")
	out = reQuerySecret.ReplaceAllString(out, "${1}[REDACTED]")
	out = reBearerToken.ReplaceAllString(out, "${1}[REDACTED]")
	out = reBasicAuthHeader.ReplaceAllString(out, "${1}[REDACTED]")
	out = reJSONSecretField.ReplaceAllString(out, "${1}[REDACTED]${3}")
	out = reEncryptedBlob.ReplaceAllString(out, "[REDACTED_ENC]")
	return out
}

