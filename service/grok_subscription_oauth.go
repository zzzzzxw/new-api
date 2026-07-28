package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	grokOAuthDefaultClientID     = "b1a00492-073a-47ea-816f-4c329264a828"
	grokOAuthDefaultAuthorizeURL = "https://auth.x.ai/oauth2/authorize"
	grokOAuthDefaultTokenURL     = "https://auth.x.ai/oauth2/token"
	grokOAuthDefaultRedirectURI  = "http://127.0.0.1:56121/callback"
	grokOAuthDefaultScope        = "openid profile email offline_access grok-cli:access api:access"
	grokOAuthSessionTTL          = 30 * time.Minute
	grokOAuthDefaultTokenTTL     = 6 * time.Hour
)

type grokOAuthSession struct {
	UserID       int
	State        string
	CodeVerifier string
	RedirectURI  string
	CreatedAt    time.Time
}

type GrokOAuthAuthorization struct {
	AuthorizationURL string `json:"authorization_url"`
	SessionID        string `json:"session_id"`
	State            string `json:"state"`
	RedirectURI      string `json:"redirect_uri"`
}

type grokOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type GrokSubscriptionOAuthKey struct {
	AccessToken       string `json:"access_token,omitempty"`
	RefreshToken      string `json:"refresh_token,omitempty"`
	IDToken           string `json:"id_token,omitempty"`
	TokenType         string `json:"token_type,omitempty"`
	ClientID          string `json:"client_id,omitempty"`
	Scope             string `json:"scope,omitempty"`
	Email             string `json:"email,omitempty"`
	Subject           string `json:"sub,omitempty"`
	TeamID            string `json:"team_id,omitempty"`
	SubscriptionTier  string `json:"subscription_tier,omitempty"`
	EntitlementStatus string `json:"entitlement_status,omitempty"`
	LastRefresh       string `json:"last_refresh,omitempty"`
	Expired           string `json:"expired,omitempty"`
	Type              string `json:"type,omitempty"`
}

func parseGrokSubscriptionOAuthKey(raw string) (*GrokSubscriptionOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("grok subscription channel: empty oauth key")
	}
	var key GrokSubscriptionOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("grok subscription channel: invalid oauth key json")
	}
	return &key, nil
}

var grokOAuthSessions = struct {
	sync.Mutex
	items map[string]grokOAuthSession
}{items: make(map[string]grokOAuthSession)}

func grokOAuthEnvOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func grokOAuthClientID() string {
	return grokOAuthEnvOrDefault("XAI_OAUTH_CLIENT_ID", grokOAuthDefaultClientID)
}

func grokOAuthTokenURL() string {
	return grokOAuthEnvOrDefault("XAI_OAUTH_TOKEN_URL", grokOAuthDefaultTokenURL)
}

func generateGrokOAuthRandom(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func GenerateGrokOAuthAuthorization(userID int) (*GrokOAuthAuthorization, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user")
	}
	state, err := generateGrokOAuthRandom(32)
	if err != nil {
		return nil, err
	}
	nonce, err := generateGrokOAuthRandom(16)
	if err != nil {
		return nil, err
	}
	sessionID, err := generateGrokOAuthRandom(24)
	if err != nil {
		return nil, err
	}
	verifier, err := generateGrokOAuthRandom(32)
	if err != nil {
		return nil, err
	}
	challengeRaw := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeRaw[:])
	redirectURI := grokOAuthEnvOrDefault("XAI_OAUTH_REDIRECT_URI", grokOAuthDefaultRedirectURI)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", grokOAuthClientID())
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", grokOAuthEnvOrDefault("XAI_OAUTH_SCOPE", grokOAuthDefaultScope))
	params.Set("state", state)
	params.Set("nonce", nonce)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("plan", "generic")
	params.Set("referrer", "new-api")

	now := time.Now()
	grokOAuthSessions.Lock()
	for id, session := range grokOAuthSessions.items {
		if now.Sub(session.CreatedAt) > grokOAuthSessionTTL {
			delete(grokOAuthSessions.items, id)
		}
	}
	grokOAuthSessions.items[sessionID] = grokOAuthSession{
		UserID:       userID,
		State:        state,
		CodeVerifier: verifier,
		RedirectURI:  redirectURI,
		CreatedAt:    now,
	}
	grokOAuthSessions.Unlock()

	authorizeURL := grokOAuthEnvOrDefault("XAI_OAUTH_AUTHORIZE_URL", grokOAuthDefaultAuthorizeURL)
	return &GrokOAuthAuthorization{
		AuthorizationURL: authorizeURL + "?" + params.Encode(),
		SessionID:        sessionID,
		State:            state,
		RedirectURI:      redirectURI,
	}, nil
}

func parseGrokOAuthCallback(raw string) (code, state string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	if parsed, err := url.Parse(trimmed); err == nil {
		if value := strings.TrimSpace(parsed.Query().Get("code")); value != "" {
			return value, strings.TrimSpace(parsed.Query().Get("state"))
		}
	}
	query := strings.TrimPrefix(trimmed, "?")
	if values, err := url.ParseQuery(query); err == nil {
		if value := strings.TrimSpace(values.Get("code")); value != "" {
			return value, strings.TrimSpace(values.Get("state"))
		}
	}
	return trimmed, ""
}

func ExchangeGrokOAuthCode(ctx context.Context, userID int, sessionID, callback, providedState, proxyURL string) (*GrokSubscriptionOAuthKey, error) {
	grokOAuthSessions.Lock()
	session, ok := grokOAuthSessions.items[strings.TrimSpace(sessionID)]
	if ok {
		delete(grokOAuthSessions.items, strings.TrimSpace(sessionID))
	}
	grokOAuthSessions.Unlock()
	if !ok || time.Since(session.CreatedAt) > grokOAuthSessionTTL {
		return nil, errors.New("oauth session not found or expired")
	}
	if session.UserID != userID {
		return nil, errors.New("oauth session does not belong to this user")
	}

	code, callbackState := parseGrokOAuthCallback(callback)
	if code == "" {
		return nil, errors.New("authorization code is required")
	}
	state := strings.TrimSpace(providedState)
	if state == "" {
		state = callbackState
	}
	if state == "" {
		return nil, errors.New("oauth state is required")
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(session.State)) != 1 {
		return nil, errors.New("invalid oauth state")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", grokOAuthClientID())
	form.Set("code", code)
	form.Set("redirect_uri", session.RedirectURI)
	form.Set("code_verifier", session.CodeVerifier)

	token, err := requestGrokOAuthToken(ctx, form, strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	return grokOAuthKeyFromToken(token, nil), nil
}

func requestGrokOAuthToken(ctx context.Context, form url.Values, proxyURL string) (*grokOAuthTokenResponse, error) {
	client, err := getCodexOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, grokOAuthTokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api-grok-oauth/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload grokOAuthTokenResponse
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xAI oauth token request failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, errors.New("xAI oauth response missing access_token")
	}
	return &payload, nil
}

func RefreshGrokOAuthToken(ctx context.Context, refreshToken, proxyURL, clientID string) (*GrokSubscriptionOAuthKey, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("empty refresh_token")
	}
	if strings.TrimSpace(clientID) == "" {
		clientID = grokOAuthClientID()
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("refresh_token", refreshToken)
	token, err := requestGrokOAuthToken(ctx, form, proxyURL)
	if err != nil {
		return nil, err
	}
	return grokOAuthKeyFromToken(token, &GrokSubscriptionOAuthKey{
		RefreshToken: refreshToken,
		ClientID:     clientID,
	}), nil
}

func grokOAuthKeyFromToken(token *grokOAuthTokenResponse, existing *GrokSubscriptionOAuthKey) *GrokSubscriptionOAuthKey {
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int64(grokOAuthDefaultTokenTTL.Seconds())
	}
	key := &GrokSubscriptionOAuthKey{
		AccessToken:  strings.TrimSpace(token.AccessToken),
		RefreshToken: strings.TrimSpace(token.RefreshToken),
		IDToken:      strings.TrimSpace(token.IDToken),
		TokenType:    strings.TrimSpace(token.TokenType),
		ClientID:     grokOAuthClientID(),
		Scope:        strings.TrimSpace(token.Scope),
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expired:      time.Now().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
		Type:         "grok_subscription",
	}
	if key.TokenType == "" {
		key.TokenType = "Bearer"
	}
	if existing != nil {
		if key.RefreshToken == "" {
			key.RefreshToken = existing.RefreshToken
		}
		if existing.ClientID != "" {
			key.ClientID = existing.ClientID
		}
		key.Email = existing.Email
		key.Subject = existing.Subject
		key.TeamID = existing.TeamID
		key.SubscriptionTier = existing.SubscriptionTier
		key.EntitlementStatus = existing.EntitlementStatus
	}
	for _, rawToken := range []string{key.IDToken, key.AccessToken} {
		claims, ok := decodeJWTClaims(rawToken)
		if !ok {
			continue
		}
		if key.Email == "" {
			key.Email, _ = claims["email"].(string)
		}
		if key.Subject == "" {
			key.Subject, _ = claims["sub"].(string)
		}
		if key.TeamID == "" {
			key.TeamID, _ = claims["team_id"].(string)
		}
		if key.SubscriptionTier == "" {
			key.SubscriptionTier, _ = claims["subscription_tier"].(string)
		}
		if key.EntitlementStatus == "" {
			key.EntitlementStatus, _ = claims["entitlement_status"].(string)
		}
	}
	return key
}
