package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type zhipuUsageEndpointResult struct {
	Status int `json:"status"`
	Data   any `json:"data"`
}

type zhipuUsageSummary struct {
	Balance           *float64 `json:"balance,omitempty"`
	TotalQuota        *float64 `json:"total_quota,omitempty"`
	UsedQuota         *float64 `json:"used_quota,omitempty"`
	SubscriptionCount int      `json:"subscription_count"`
	ModelUsageCount   int      `json:"model_usage_count"`
	PerformanceCount  int      `json:"performance_count"`
}

func GetZhipuChannelUsage(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeZhipu && ch.Type != constant.ChannelTypeZhipu_v4 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Zhipu"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	headers, err := getZhipuDashboardAuthHeader(ch)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	startTime, endTime := zhipuUsageTimeRange(c.Query("range"), time.Now())
	baseURL := "https://www.bigmodel.cn"
	endpoints := map[string]string{
		"quota_limit":       baseURL + "/api/monitor/usage/quota/limit",
		"subscription_list": baseURL + "/api/biz/subscription/list?pageSize=9999&pageNum=1",
		"model_usage": baseURL + "/api/monitor/usage/model-usage?" + url.Values{
			"startTime": []string{startTime.Format("2006-01-02 15:04:05")},
			"endTime":   []string{endTime.Format("2006-01-02 15:04:05")},
		}.Encode(),
		"model_performance_day": baseURL + "/api/monitor/usage/model-performance-day?" + url.Values{
			"startTime": []string{startTime.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
			"endTime":   []string{endTime.AddDate(0, 0, -1).Format("2006-01-02 15:04:05")},
		}.Encode(),
	}

	results := make(map[string]zhipuUsageEndpointResult, len(endpoints))
	summary := zhipuUsageSummary{}
	var firstErr error

	for name, endpoint := range endpoints {
		status, payload, err := fetchZhipuDashboardEndpoint(c.Request.Context(), client, endpoint, headers)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			results[name] = zhipuUsageEndpointResult{
				Status: status,
				Data:   gin.H{"error": err.Error()},
			}
			continue
		}
		results[name] = zhipuUsageEndpointResult{Status: status, Data: payload}
		updateZhipuUsageSummary(&summary, name, payload)
	}

	if summary.Balance != nil {
		ch.UpdateBalance(*summary.Balance)
	}

	success := firstErr == nil
	message := ""
	if firstErr != nil {
		message = firstErr.Error()
	}
	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": message,
		"data": gin.H{
			"summary":   summary,
			"endpoints": results,
		},
	})
}

func zhipuUsageTimeRange(rangeKey string, now time.Time) (time.Time, time.Time) {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	switch rangeKey {
	case "today":
		return todayStart, todayEnd
	case "30d":
		return todayStart.AddDate(0, 0, -29), todayEnd
	default:
		return todayStart.AddDate(0, 0, -6), todayEnd
	}
}

func fetchZhipuDashboardEndpoint(ctx context.Context, client *http.Client, endpoint string, headers http.Header) (int, any, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	for key := range headers {
		req.Header.Set(key, headers.Get(key))
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		payload = string(body)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, payload, fmt.Errorf("upstream status: %d", resp.StatusCode)
	}
	return resp.StatusCode, payload, nil
}

func updateZhipuUsageSummary(summary *zhipuUsageSummary, name string, payload any) {
	metric := scanZhipuDashboardValue("", payload)
	if metric.Found && (summary.Balance == nil || metric.Value > *summary.Balance) {
		value := metric.Value
		summary.Balance = &value
	}
	if name == "subscription_list" {
		summary.SubscriptionCount = countZhipuItems(payload)
		return
	}
	if name == "model_usage" {
		summary.ModelUsageCount = countZhipuItems(payload)
		return
	}
	if name == "model_performance_day" {
		summary.PerformanceCount = countZhipuItems(payload)
	}
	if totals := zhipuCollectTotalUsed(payload); totals.totalFound && totals.usedFound {
		total := totals.total
		used := totals.used
		summary.TotalQuota = &total
		summary.UsedQuota = &used
		if summary.Balance == nil && total >= used {
			balance := total - used
			summary.Balance = &balance
		}
	}
}

func countZhipuItems(payload any) int {
	switch typed := payload.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		best := 0
		for _, value := range typed {
			if count := countZhipuItems(value); count > best {
				best = count
			}
		}
		return best
	default:
		return 0
	}
}

type zhipuTotalUsed struct {
	total      float64
	used       float64
	totalFound bool
	usedFound  bool
}

func zhipuCollectTotalUsed(payload any) zhipuTotalUsed {
	switch typed := payload.(type) {
	case map[string]any:
		result := zhipuTotalUsed{}
		for key, value := range typed {
			if number, ok := zhipuNumber(value); ok {
				lowerKey := key
				if zhipuLooksLikeTotalKey(lowerKey) && (!result.totalFound || number > result.total) {
					result.total = number
					result.totalFound = true
				}
				if zhipuLooksLikeUsedKey(lowerKey) && (!result.usedFound || number > result.used) {
					result.used = number
					result.usedFound = true
				}
			}
			child := zhipuCollectTotalUsed(value)
			if child.totalFound && (!result.totalFound || child.total > result.total) {
				result.total = child.total
				result.totalFound = true
			}
			if child.usedFound && (!result.usedFound || child.used > result.used) {
				result.used = child.used
				result.usedFound = true
			}
		}
		return result
	case []any:
		result := zhipuTotalUsed{}
		for _, item := range typed {
			child := zhipuCollectTotalUsed(item)
			if child.totalFound && (!result.totalFound || child.total > result.total) {
				result.total = child.total
				result.totalFound = true
			}
			if child.usedFound && (!result.usedFound || child.used > result.used) {
				result.used = child.used
				result.usedFound = true
			}
		}
		return result
	default:
		return zhipuTotalUsed{}
	}
}
