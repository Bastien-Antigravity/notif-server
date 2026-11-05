package main

// =============================================================================
// ESSENTIAL PROCESS:
// Unit tests for notif-server main entrypoint helpers, including pre-flight
// Telegram Bot Token verification.
//
// DATA FLOW:
// Mock Telegram API HTTP Server -> verifyTelegramToken() -> assertions
//
// KEY PARAMETERS:
// - serverOk: Mock HTTP server returning 200 OK with bot identity.
// - serverFail: Mock HTTP server returning 401 Unauthorized.
// =============================================================================

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------

func TestVerifyTelegramToken(t *testing.T) {
	// 1. Success case: server returns ok: true
	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/bot123456:VALID_TOKEN/getMe", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot", "username": "TestAlertBot"}}`))
	}))
	defer serverOk.Close()

	ok, username, err := verifyTelegramToken(serverOk.URL, "123456:VALID_TOKEN")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "@TestAlertBot", username)

	// 2. Failure case: server returns ok: false
	serverFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok": false, "error_code": 401, "description": "Unauthorized"}`))
	}))
	defer serverFail.Close()

	ok, desc, err := verifyTelegramToken(serverFail.URL, "invalid_token")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "Unauthorized", desc)

	// 3. Empty token
	_, _, err = verifyTelegramToken(serverOk.URL, "   ")
	assert.Error(t, err)
}
