package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func RefreshGrokSubscriptionChannelCredential(ctx context.Context, channelID int, resetCaches bool) (*GrokSubscriptionOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeGrokSubscription {
		return nil, nil, fmt.Errorf("channel type is not Grok Subscription")
	}

	current, err := parseGrokSubscriptionOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(current.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("grok subscription channel: refresh_token is required")
	}

	refreshed, err := RefreshGrokOAuthToken(ctx, current.RefreshToken, ch.GetSetting().Proxy, current.ClientID)
	if err != nil {
		return nil, nil, err
	}
	if refreshed.Email == "" {
		refreshed.Email = current.Email
	}
	if refreshed.Subject == "" {
		refreshed.Subject = current.Subject
	}
	if refreshed.TeamID == "" {
		refreshed.TeamID = current.TeamID
	}
	if refreshed.SubscriptionTier == "" {
		refreshed.SubscriptionTier = current.SubscriptionTier
	}
	if refreshed.EntitlementStatus == "" {
		refreshed.EntitlementStatus = current.EntitlementStatus
	}

	encoded, err := common.Marshal(refreshed)
	if err != nil {
		return nil, nil, err
	}
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Update("key", string(encoded)).Error; err != nil {
		return nil, nil, err
	}
	if resetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}
	return refreshed, ch, nil
}

func grokSubscriptionCredentialNeedsRefresh(raw string, now time.Time) bool {
	key, err := parseGrokSubscriptionOAuthKey(raw)
	if err != nil || strings.TrimSpace(key.RefreshToken) == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(key.Expired))
	return err != nil || expiresAt.Sub(now) <= 15*time.Minute
}
