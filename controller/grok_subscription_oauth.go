package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel/groksubscription"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GenerateGrokSubscriptionAuthorization(c *gin.Context) {
	result, err := service.GenerateGrokOAuthAuthorization(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

type grokSubscriptionExchangeRequest struct {
	SessionID string `json:"session_id"`
	Callback  string `json:"callback"`
	State     string `json:"state"`
	Proxy     string `json:"proxy"`
}

func ExchangeGrokSubscriptionAuthorization(c *gin.Context) {
	var request grokSubscriptionExchangeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	key, err := service.ExchangeGrokOAuthCode(ctx, c.GetInt("id"), request.SessionID, request.Callback, request.State, request.Proxy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	credential, err := common.Marshal(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"credential": string(credential),
		"email":      key.Email,
		"expires_at": key.Expired,
		"models":     groksubscription.ModelList,
	})
}

func RefreshGrokSubscriptionChannelCredential(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	key, ch, err := service.RefreshGrokSubscriptionChannelCredential(ctx, channelID, true)
	if err != nil {
		common.SysError("failed to refresh grok subscription credential: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "刷新 Grok 凭证失败，请稍后重试"})
		return
	}
	common.ApiSuccess(c, gin.H{
		"expires_at":   key.Expired,
		"last_refresh": key.LastRefresh,
		"email":        strings.TrimSpace(key.Email),
		"channel_id":   ch.Id,
		"channel_type": ch.Type,
		"channel_name": ch.Name,
	})
}
