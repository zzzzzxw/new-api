package groksubscription

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type OAuthKey struct {
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

func ParseOAuthKey(raw string) (*OAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("grok subscription channel: empty oauth key")
	}
	var key OAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("grok subscription channel: invalid oauth key json")
	}
	return &key, nil
}
