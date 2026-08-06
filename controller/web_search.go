package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

const webSearchToolName = "web_search"

// WebSearch handles POST /v1/search. It is authenticated by the platform API
// key (TokenAuth) and proxies queries to DuckDuckGo's Instant Answer API.
// Each call is billed at the configured web_search tool price.
func WebSearch(c *gin.Context) {
	var req dto.WebSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request body: " + err.Error(),
		})
		return
	}
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "query is required",
		})
		return
	}

	maxResults := 5
	if req.MaxResults != nil {
		maxResults = *req.MaxResults
	}
	if maxResults < 1 {
		maxResults = 1
	}
	if maxResults > 20 {
		maxResults = 20
	}

	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	tokenName := c.GetString("token_name")
	tokenUnlimited := c.GetBool("token_unlimited_quota")
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if userGroup == "" {
		userGroup = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}

	// Billing: one web_search tool call, same pricing as model-embedded search.
	billing := service.ComputeToolCallQuota(service.ToolCallUsage{
		WebSearchCalls:    1,
		WebSearchToolName: webSearchToolName,
	}, ratio_setting.GetGroupRatio(userGroup))
	quota := billing.TotalQuota

	if quota > 0 && !tokenUnlimited {
		userQuota := common.GetContextKeyInt(c, constant.ContextKeyUserQuota)
		if userQuota < quota {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "insufficient quota",
			})
			return
		}
	}

	result, err := service.SearchDuckDuckGo(req.Query, maxResults)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "search failed: " + err.Error(),
		})
		return
	}

	if quota > 0 {
		if err := model.DecreaseUserQuota(userId, quota, false); err != nil {
			common.SysLog("web search decrease user quota error: " + err.Error())
		}
		if !tokenUnlimited {
			if err := model.DecreaseTokenQuota(tokenId, c.GetString("token_key"), quota); err != nil {
				common.SysLog("web search decrease token quota error: " + err.Error())
			}
		}
		model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
			ModelName: "web_search",
			TokenName: tokenName,
			TokenId:   tokenId,
			Quota:     quota,
			Group:     userGroup,
			Content:   "web search: " + req.Query,
			Other: map[string]interface{}{
				"tool":     webSearchToolName,
				"query":    req.Query,
				"provider": "duckduckgo",
				"price_1k": operation_setting.GetToolPrice(webSearchToolName),
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
