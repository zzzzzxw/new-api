package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchGrokSubscriptionUsageEndpointsUseOfficialCLIContract(t *testing.T) {
	t.Setenv("XAI_GROK_CLI_VERSION", "1.2.3")

	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		assert.Equal(t, "xai-grok-cli", r.Header.Get("X-XAI-Token-Auth"))
		assert.Equal(t, "1.2.3", r.Header.Get("x-grok-client-version"))
		assert.Equal(t, "interactive", r.Header.Get("x-grok-client-mode"))
		if r.URL.Path != "/v1/user" {
			assert.Equal(t, "user-123", r.Header.Get("x-userid"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	status, _, err := FetchGrokSubscriptionUser(
		context.Background(),
		server.Client(),
		server.URL,
		"access-token",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)

	status, _, err = FetchGrokSubscriptionBilling(
		context.Background(),
		server.Client(),
		server.URL+"/v1",
		"access-token",
		"user-123",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)

	status, _, err = FetchGrokSubscriptionAutoTopupRule(
		context.Background(),
		server.Client(),
		server.URL,
		"access-token",
		"user-123",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)

	assert.Equal(t, []string{
		"/v1/user?include=subscription",
		"/v1/billing?format=credits",
		"/v1/auto-topup-rule",
	}, requested)
}

func TestGrokSubscriptionUsageURLRejectsInvalidBaseURL(t *testing.T) {
	_, err := grokSubscriptionUsageURL("cli-chat-proxy.grok.com", "/user", nil)
	require.Error(t, err)
}
