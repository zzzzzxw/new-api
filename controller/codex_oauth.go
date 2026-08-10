package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const codexOAuthSessionTTL = 10 * time.Minute

type codexOAuthCompleteRequest struct {
	Input string `json:"input"`
}

func codexOAuthSessionKey(channelID int, field string) string {
	return fmt.Sprintf("codex_oauth_%s_%d", field, channelID)
}

func parseCodexAuthorizationInput(input string) (string, string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", errors.New("empty input")
	}

	if strings.Contains(value, "#") && !strings.Contains(value, "://") {
		parts := strings.SplitN(value, "#", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	}

	if strings.Contains(value, "code=") {
		parsedURL, parseErr := url.Parse(value)
		if parseErr == nil {
			query := parsedURL.Query()
			if code := strings.TrimSpace(query.Get("code")); code != "" {
				return code, strings.TrimSpace(query.Get("state")), nil
			}
		}
		query, parseErr := url.ParseQuery(value)
		if parseErr == nil {
			return strings.TrimSpace(query.Get("code")), strings.TrimSpace(query.Get("state")), nil
		}
	}

	return value, "", nil
}

func StartCodexOAuthForChannel(c *gin.Context) {
	channelID, err := parseCodexChannelID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	_, err = getCodexOAuthChannel(channelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	flow, err := service.CreateCodexOAuthAuthorizationFlow()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	session.Set(codexOAuthSessionKey(channelID, "state"), flow.State)
	session.Set(codexOAuthSessionKey(channelID, "verifier"), flow.Verifier)
	session.Set(codexOAuthSessionKey(channelID, "created_at"), time.Now().Unix())
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"authorize_url": flow.AuthorizeURL},
	})
}

func CompleteCodexOAuthForChannel(c *gin.Context) {
	channelID, err := parseCodexChannelID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := getCodexOAuthChannel(channelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var request codexOAuthCompleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	code, state, err := parseCodexAuthorizationInput(request.Input)
	if err != nil || code == "" || state == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "授权回调必须包含 code 和 state"})
		return
	}

	session := sessions.Default(c)
	expectedState, _ := session.Get(codexOAuthSessionKey(channelID, "state")).(string)
	verifier, _ := session.Get(codexOAuthSessionKey(channelID, "verifier")).(string)
	createdAt := sessionUnixValue(session.Get(codexOAuthSessionKey(channelID, "created_at")))
	if expectedState == "" || verifier == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "OAuth 流程未启动或会话已过期"})
		return
	}
	if createdAt == 0 || time.Since(time.Unix(createdAt, 0)) > codexOAuthSessionTTL {
		clearCodexOAuthSession(session, channelID)
		_ = session.Save()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "OAuth 会话已过期，请重新开始授权"})
		return
	}
	if state != expectedState {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "OAuth state 校验失败"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	tokenResult, err := service.ExchangeCodexAuthorizationCodeWithProxy(
		ctx,
		code,
		verifier,
		channel.GetSetting().Proxy,
	)
	if err != nil {
		common.SysError("failed to exchange codex authorization code: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "授权码交换失败，请重试"})
		return
	}

	accountID, ok := service.ExtractCodexAccountIDFromJWT(tokenResult.AccessToken)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无法从授权结果中读取 account_id"})
		return
	}
	email, _ := service.ExtractEmailFromJWT(tokenResult.AccessToken)
	key := service.CodexOAuthKey{
		AccessToken:  tokenResult.AccessToken,
		RefreshToken: tokenResult.RefreshToken,
		AccountID:    accountID,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Email:        email,
		Type:         "codex",
		Expired:      tokenResult.ExpiresAt.Format(time.RFC3339),
	}
	encodedKey, err := common.Marshal(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Model(&model.Channel{}).
		Where("id = ?", channelID).
		Update("key", string(encodedKey)).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	clearCodexOAuthSession(session, channelID)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "saved",
		"data": gin.H{
			"channel_id":   channelID,
			"account_id":   accountID,
			"email":        email,
			"expires_at":   key.Expired,
			"last_refresh": key.LastRefresh,
		},
	})
}

func parseCodexChannelID(c *gin.Context) (int, error) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		return 0, fmt.Errorf("invalid channel id")
	}
	return channelID, nil
}

func getCodexOAuthChannel(channelID int) (*model.Channel, error) {
	channel, err := model.GetChannelById(channelID, false)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	if channel.Type != constant.ChannelTypeCodex {
		return nil, fmt.Errorf("channel type is not Codex")
	}
	return channel, nil
}

func sessionUnixValue(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func clearCodexOAuthSession(session sessions.Session, channelID int) {
	session.Delete(codexOAuthSessionKey(channelID, "state"))
	session.Delete(codexOAuthSessionKey(channelID, "verifier"))
	session.Delete(codexOAuthSessionKey(channelID, "created_at"))
}
