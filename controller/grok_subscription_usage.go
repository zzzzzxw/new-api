package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/groksubscription"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type grokSubscriptionUsageEndpointResult struct {
	Status int `json:"status"`
	Data   any `json:"data"`
}

type grokSubscriptionUsageSummary struct {
	UserID                    string   `json:"user_id,omitempty"`
	Email                     string   `json:"email,omitempty"`
	FirstName                 string   `json:"first_name,omitempty"`
	LastName                  string   `json:"last_name,omitempty"`
	PrincipalType             string   `json:"principal_type,omitempty"`
	TeamID                    string   `json:"team_id,omitempty"`
	TeamName                  string   `json:"team_name,omitempty"`
	OrganizationID            string   `json:"organization_id,omitempty"`
	OrganizationName          string   `json:"organization_name,omitempty"`
	SubscriptionTier          string   `json:"subscription_tier,omitempty"`
	EntitlementStatus         string   `json:"entitlement_status,omitempty"`
	CredentialExpiresAt       string   `json:"credential_expires_at,omitempty"`
	LastCredentialRefresh     string   `json:"last_credential_refresh,omitempty"`
	CreditUsagePercent        *float64 `json:"credit_usage_percent,omitempty"`
	UsagePeriodType           string   `json:"usage_period_type,omitempty"`
	UsagePeriodStart          string   `json:"usage_period_start,omitempty"`
	UsagePeriodEnd            string   `json:"usage_period_end,omitempty"`
	MonthlyLimitCents         *int64   `json:"monthly_limit_cents,omitempty"`
	UsedCents                 *int64   `json:"used_cents,omitempty"`
	OnDemandCapCents          *int64   `json:"on_demand_cap_cents,omitempty"`
	OnDemandUsedCents         *int64   `json:"on_demand_used_cents,omitempty"`
	PrepaidBalanceCents       *int64   `json:"prepaid_balance_cents,omitempty"`
	IsUnifiedBillingUser      *bool    `json:"is_unified_billing_user,omitempty"`
	AutoTopupEnabled          *bool    `json:"auto_topup_enabled,omitempty"`
	AutoTopupAmountCents      *int64   `json:"auto_topup_amount_cents,omitempty"`
	AutoTopupMonthlyMaxCents  *int64   `json:"auto_topup_monthly_max_cents,omitempty"`
	CodingDataRetentionOptOut *bool    `json:"coding_data_retention_opt_out,omitempty"`
}

type grokSubscriptionUserInfo struct {
	UserID                    string `json:"userId"`
	Email                     string `json:"email"`
	FirstName                 string `json:"firstName"`
	LastName                  string `json:"lastName"`
	PrincipalType             string `json:"principalType"`
	TeamID                    string `json:"teamId"`
	TeamName                  string `json:"teamName"`
	OrganizationID            string `json:"organizationId"`
	OrganizationName          string `json:"organizationName"`
	SubscriptionTier          string `json:"subscriptionTier"`
	CodingDataRetentionOptOut *bool  `json:"codingDataRetentionOptOut"`
}

type grokSubscriptionCent struct {
	Val int64 `json:"val"`
}

type grokSubscriptionUsagePeriod struct {
	Type  string `json:"type"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type grokSubscriptionBillingConfig struct {
	CreditUsagePercent   *float64                     `json:"creditUsagePercent"`
	CurrentPeriod        *grokSubscriptionUsagePeriod `json:"currentPeriod"`
	MonthlyLimit         *grokSubscriptionCent        `json:"monthlyLimit"`
	Used                 *grokSubscriptionCent        `json:"used"`
	OnDemandCap          *grokSubscriptionCent        `json:"onDemandCap"`
	OnDemandUsed         *grokSubscriptionCent        `json:"onDemandUsed"`
	PrepaidBalance       *grokSubscriptionCent        `json:"prepaidBalance"`
	IsUnifiedBillingUser *bool                        `json:"isUnifiedBillingUser"`
	BillingPeriodStart   string                       `json:"billingPeriodStart"`
	BillingPeriodEnd     string                       `json:"billingPeriodEnd"`
}

type grokSubscriptionBillingResponse struct {
	Config           *grokSubscriptionBillingConfig `json:"config"`
	SubscriptionTier string                         `json:"subscriptionTier"`
}

type grokSubscriptionAutoTopupRule struct {
	Enabled           bool                  `json:"enabled"`
	TopupAmount       *grokSubscriptionCent `json:"topupAmount"`
	MaxAmountPerMonth *grokSubscriptionCent `json:"maxAmountPerMonth"`
}

type grokSubscriptionAutoTopupResponse struct {
	Rule *grokSubscriptionAutoTopupRule `json:"rule"`
}

type grokSubscriptionUsageFetchResult struct {
	Summary      grokSubscriptionUsageSummary
	Endpoints    map[string]grokSubscriptionUsageEndpointResult
	Errors       []string
	Unauthorized bool
	Success      bool
}

func GetGrokSubscriptionChannelUsage(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if channel.Type != constant.ChannelTypeGrokSubscription {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Grok Subscription"})
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	oauthKey, err := groksubscription.ParseOAuthKey(strings.TrimSpace(channel.Key))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析 Grok 凭证失败，请检查渠道配置"})
		return
	}
	if strings.TrimSpace(oauthKey.AccessToken) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "grok subscription channel: access_token is required"})
		return
	}

	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	baseURL := channel.GetBaseURL()
	if strings.TrimSpace(baseURL) == "" {
		baseURL = constant.ChannelBaseURLs[channel.Type]
	}
	result := fetchGrokSubscriptionUsage(
		ctx,
		client,
		baseURL,
		oauthKey.AccessToken,
		oauthKey.Subject,
	)

	if result.Unauthorized && strings.TrimSpace(oauthKey.RefreshToken) != "" {
		refreshCtx, refreshCancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		refreshed, _, refreshErr := service.RefreshGrokSubscriptionChannelCredential(refreshCtx, channel.Id, true)
		refreshCancel()
		if refreshErr == nil {
			oauthKey.AccessToken = refreshed.AccessToken
			oauthKey.Subject = refreshed.Subject
			oauthKey.Email = refreshed.Email
			oauthKey.SubscriptionTier = refreshed.SubscriptionTier
			oauthKey.EntitlementStatus = refreshed.EntitlementStatus
			oauthKey.Expired = refreshed.Expired
			oauthKey.LastRefresh = refreshed.LastRefresh

			retryCtx, retryCancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
			result = fetchGrokSubscriptionUsage(
				retryCtx,
				client,
				baseURL,
				oauthKey.AccessToken,
				oauthKey.Subject,
			)
			retryCancel()
		} else {
			result.Errors = append(result.Errors, "refresh credential: "+refreshErr.Error())
		}
	}

	applyGrokCredentialSummary(&result.Summary, oauthKey)
	c.JSON(http.StatusOK, gin.H{
		"success": result.Success,
		"message": strings.Join(result.Errors, "; "),
		"data": gin.H{
			"summary":   result.Summary,
			"endpoints": result.Endpoints,
		},
	})
}

func fetchGrokSubscriptionUsage(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
	fallbackUserID string,
) grokSubscriptionUsageFetchResult {
	result := grokSubscriptionUsageFetchResult{
		Endpoints: make(map[string]grokSubscriptionUsageEndpointResult, 3),
	}

	userStatus, userBody, userErr := service.FetchGrokSubscriptionUser(
		ctx,
		client,
		baseURL,
		accessToken,
	)
	userPayload := decodeGrokSubscriptionPayload(userBody)
	result.Endpoints["user"] = grokSubscriptionUsageEndpointResult{
		Status: userStatus,
		Data:   userPayload,
	}
	if userErr != nil {
		result.Errors = append(result.Errors, "user: "+userErr.Error())
	} else if !isHTTPSuccess(userStatus) {
		result.Errors = append(result.Errors, fmt.Sprintf("user: upstream status %d", userStatus))
	} else {
		result.Success = true
		var userInfo grokSubscriptionUserInfo
		if common.Unmarshal(userBody, &userInfo) == nil {
			applyGrokUserSummary(&result.Summary, &userInfo)
		}
	}
	result.Unauthorized = isUnauthorizedStatus(userStatus)

	userID := strings.TrimSpace(result.Summary.UserID)
	if userID == "" {
		userID = strings.TrimSpace(fallbackUserID)
		result.Summary.UserID = userID
	}
	if userID == "" {
		result.Errors = append(result.Errors, "billing: user id is unavailable")
		return result
	}

	billingStatus, billingBody, billingErr := service.FetchGrokSubscriptionBilling(
		ctx,
		client,
		baseURL,
		accessToken,
		userID,
	)
	billingPayload := decodeGrokSubscriptionPayload(billingBody)
	result.Endpoints["billing"] = grokSubscriptionUsageEndpointResult{
		Status: billingStatus,
		Data:   billingPayload,
	}
	if billingErr != nil {
		result.Errors = append(result.Errors, "billing: "+billingErr.Error())
	} else if !isHTTPSuccess(billingStatus) {
		result.Errors = append(result.Errors, fmt.Sprintf("billing: upstream status %d", billingStatus))
	} else {
		result.Success = true
		var billing grokSubscriptionBillingResponse
		if common.Unmarshal(billingBody, &billing) == nil {
			applyGrokBillingSummary(&result.Summary, &billing)
		}
	}
	result.Unauthorized = result.Unauthorized || isUnauthorizedStatus(billingStatus)

	autoTopupStatus, autoTopupBody, autoTopupErr := service.FetchGrokSubscriptionAutoTopupRule(
		ctx,
		client,
		baseURL,
		accessToken,
		userID,
	)
	autoTopupPayload := decodeGrokSubscriptionPayload(autoTopupBody)
	result.Endpoints["auto_topup_rule"] = grokSubscriptionUsageEndpointResult{
		Status: autoTopupStatus,
		Data:   autoTopupPayload,
	}
	if autoTopupErr != nil {
		result.Errors = append(result.Errors, "auto top-up: "+autoTopupErr.Error())
	} else if !isHTTPSuccess(autoTopupStatus) {
		result.Errors = append(result.Errors, fmt.Sprintf("auto top-up: upstream status %d", autoTopupStatus))
	} else {
		var autoTopup grokSubscriptionAutoTopupResponse
		if common.Unmarshal(autoTopupBody, &autoTopup) == nil {
			applyGrokAutoTopupSummary(&result.Summary, &autoTopup)
		}
	}
	result.Unauthorized = result.Unauthorized || isUnauthorizedStatus(autoTopupStatus)
	return result
}

func decodeGrokSubscriptionPayload(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var payload any
	if common.Unmarshal(body, &payload) != nil {
		return string(body)
	}
	return payload
}

func applyGrokCredentialSummary(summary *grokSubscriptionUsageSummary, key *groksubscription.OAuthKey) {
	if summary == nil || key == nil {
		return
	}
	if summary.UserID == "" {
		summary.UserID = strings.TrimSpace(key.Subject)
	}
	if summary.Email == "" {
		summary.Email = strings.TrimSpace(key.Email)
	}
	if summary.TeamID == "" {
		summary.TeamID = strings.TrimSpace(key.TeamID)
	}
	if summary.SubscriptionTier == "" {
		summary.SubscriptionTier = strings.TrimSpace(key.SubscriptionTier)
	}
	summary.EntitlementStatus = strings.TrimSpace(key.EntitlementStatus)
	summary.CredentialExpiresAt = strings.TrimSpace(key.Expired)
	summary.LastCredentialRefresh = strings.TrimSpace(key.LastRefresh)
}

func applyGrokUserSummary(summary *grokSubscriptionUsageSummary, user *grokSubscriptionUserInfo) {
	if summary == nil || user == nil {
		return
	}
	summary.UserID = strings.TrimSpace(user.UserID)
	summary.Email = strings.TrimSpace(user.Email)
	summary.FirstName = strings.TrimSpace(user.FirstName)
	summary.LastName = strings.TrimSpace(user.LastName)
	summary.PrincipalType = strings.TrimSpace(user.PrincipalType)
	summary.TeamID = strings.TrimSpace(user.TeamID)
	summary.TeamName = strings.TrimSpace(user.TeamName)
	summary.OrganizationID = strings.TrimSpace(user.OrganizationID)
	summary.OrganizationName = strings.TrimSpace(user.OrganizationName)
	summary.SubscriptionTier = strings.TrimSpace(user.SubscriptionTier)
	summary.CodingDataRetentionOptOut = user.CodingDataRetentionOptOut
}

func applyGrokBillingSummary(summary *grokSubscriptionUsageSummary, billing *grokSubscriptionBillingResponse) {
	if summary == nil || billing == nil {
		return
	}
	if summary.SubscriptionTier == "" {
		summary.SubscriptionTier = strings.TrimSpace(billing.SubscriptionTier)
	}
	config := billing.Config
	if config == nil {
		return
	}
	summary.CreditUsagePercent = config.CreditUsagePercent
	summary.IsUnifiedBillingUser = config.IsUnifiedBillingUser
	summary.MonthlyLimitCents = grokCentValue(config.MonthlyLimit)
	summary.UsedCents = grokCentValue(config.Used)
	summary.OnDemandCapCents = grokCentValue(config.OnDemandCap)
	summary.OnDemandUsedCents = grokCentValue(config.OnDemandUsed)
	summary.PrepaidBalanceCents = grokCentValue(config.PrepaidBalance)
	if config.CurrentPeriod != nil {
		summary.UsagePeriodType = strings.TrimSpace(config.CurrentPeriod.Type)
		summary.UsagePeriodStart = strings.TrimSpace(config.CurrentPeriod.Start)
		summary.UsagePeriodEnd = strings.TrimSpace(config.CurrentPeriod.End)
	} else {
		summary.UsagePeriodStart = strings.TrimSpace(config.BillingPeriodStart)
		summary.UsagePeriodEnd = strings.TrimSpace(config.BillingPeriodEnd)
	}
}

func applyGrokAutoTopupSummary(summary *grokSubscriptionUsageSummary, response *grokSubscriptionAutoTopupResponse) {
	if summary == nil || response == nil || response.Rule == nil {
		return
	}
	enabled := response.Rule.Enabled
	summary.AutoTopupEnabled = &enabled
	summary.AutoTopupAmountCents = grokCentValue(response.Rule.TopupAmount)
	summary.AutoTopupMonthlyMaxCents = grokCentValue(response.Rule.MaxAmountPerMonth)
}

func grokCentValue(value *grokSubscriptionCent) *int64 {
	if value == nil {
		return nil
	}
	result := value.Val
	return &result
}

func isHTTPSuccess(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

func isUnauthorizedStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}
