package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestCreateCodexOAuthAuthorizationFlowBuildsPKCEURL(t *testing.T) {
	flow, err := CreateCodexOAuthAuthorizationFlow()
	require.NoError(t, err)
	require.NotEmpty(t, flow.State)
	require.NotEmpty(t, flow.Verifier)

	parsedURL, err := url.Parse(flow.AuthorizeURL)
	require.NoError(t, err)
	query := parsedURL.Query()
	require.Equal(t, codexOAuthClientID, query.Get("client_id"))
	require.Equal(t, codexOAuthRedirectURI, query.Get("redirect_uri"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))
	require.Equal(t, flow.State, query.Get("state"))

	sum := sha256.Sum256([]byte(flow.Verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	require.Equal(t, expectedChallenge, query.Get("code_challenge"))
}

func TestExchangeCodexAuthorizationCodeSendsPKCEForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.NoError(t, request.ParseForm())
		require.Equal(t, "authorization_code", request.Form.Get("grant_type"))
		require.Equal(t, codexOAuthClientID, request.Form.Get("client_id"))
		require.Equal(t, "authorization-code", request.Form.Get("code"))
		require.Equal(t, "verifier", request.Form.Get("code_verifier"))
		require.Equal(t, codexOAuthRedirectURI, request.Form.Get("redirect_uri"))

		payload, err := common.Marshal(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
		})
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	result, err := exchangeCodexAuthorizationCode(
		context.Background(),
		server.Client(),
		server.URL,
		codexOAuthClientID,
		"authorization-code",
		"verifier",
		codexOAuthRedirectURI,
	)
	require.NoError(t, err)
	require.Equal(t, "access-token", result.AccessToken)
	require.Equal(t, "refresh-token", result.RefreshToken)
	require.NotEmpty(t, result.ExpiresAt)
}

func TestExtractCodexClaimsFromJWT(t *testing.T) {
	payload, err := common.Marshal(map[string]any{
		codexJWTClaimPath: map[string]string{
			"chatgpt_account_id": "account-123",
		},
		"email": "user@example.com",
	})
	require.NoError(t, err)

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	token := "header." + encodedPayload + ".signature"

	accountID, ok := ExtractCodexAccountIDFromJWT(token)
	require.True(t, ok)
	require.Equal(t, "account-123", accountID)

	email, ok := ExtractEmailFromJWT(token)
	require.True(t, ok)
	require.Equal(t, "user@example.com", email)
}
