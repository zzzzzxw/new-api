package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestGenerateGrokOAuthAuthorizationBuildsPKCEURL(t *testing.T) {
	t.Setenv("XAI_OAUTH_AUTHORIZE_URL", "https://auth.x.ai/oauth2/authorize")
	result, err := GenerateGrokOAuthAuthorization(123)
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionID)

	parsed, err := url.Parse(result.AuthorizationURL)
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, grokOAuthDefaultClientID, query.Get("client_id"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))
	require.NotEmpty(t, query.Get("code_challenge"))
	require.Equal(t, result.State, query.Get("state"))
	require.Contains(t, query.Get("scope"), "grok-cli:access")
}

func TestRefreshGrokOAuthTokenPreservesRotatingRefreshTokenFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		require.Equal(t, "old-refresh", r.Form.Get("refresh_token"))
		payload, err := common.Marshal(map[string]any{
			"access_token": "new-access",
			"expires_in":   3600,
			"token_type":   "Bearer",
		})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	t.Setenv("XAI_OAUTH_TOKEN_URL", server.URL)

	key, err := RefreshGrokOAuthToken(context.Background(), "old-refresh", "", "")
	require.NoError(t, err)
	require.Equal(t, "new-access", key.AccessToken)
	require.Equal(t, "old-refresh", key.RefreshToken)
	require.NotEmpty(t, key.Expired)
}

func TestExchangeGrokOAuthCodeValidatesStateAndUsesPKCEVerifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		require.Equal(t, "authorization-code", r.Form.Get("code"))
		require.NotEmpty(t, r.Form.Get("code_verifier"))
		payload, err := common.Marshal(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	t.Setenv("XAI_OAUTH_TOKEN_URL", server.URL)

	authorization, err := GenerateGrokOAuthAuthorization(321)
	require.NoError(t, err)
	callback := "http://127.0.0.1:56121/callback?code=authorization-code&state=" +
		url.QueryEscape(authorization.State)
	key, err := ExchangeGrokOAuthCode(
		context.Background(),
		321,
		authorization.SessionID,
		callback,
		authorization.State,
		"",
	)
	require.NoError(t, err)
	require.Equal(t, "access-token", key.AccessToken)
	require.Equal(t, "refresh-token", key.RefreshToken)
}
