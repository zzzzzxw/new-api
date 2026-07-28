package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

var (
	grokSubscriptionRefreshOnce    sync.Once
	grokSubscriptionRefreshRunning atomic.Bool
)

func StartGrokSubscriptionCredentialAutoRefreshTask() {
	grokSubscriptionRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			runGrokSubscriptionCredentialRefresh()
			for range ticker.C {
				runGrokSubscriptionCredentialRefresh()
			}
		})
	})
}

func runGrokSubscriptionCredentialRefresh() {
	if !grokSubscriptionRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer grokSubscriptionRefreshRunning.Store(false)

	var channels []*model.Channel
	if err := model.DB.
		Select("id", "name", "key", "status", "channel_info").
		Where("type = ? AND (status = ? OR status = ?)",
			constant.ChannelTypeGrokSubscription,
			common.ChannelStatusEnabled,
			common.ChannelStatusAutoDisabled,
		).
		Find(&channels).Error; err != nil {
		logger.LogError(context.Background(), fmt.Sprintf("grok subscription credential refresh: query failed: %v", err))
		return
	}

	refreshed := 0
	now := time.Now()
	for _, ch := range channels {
		if ch == nil || ch.ChannelInfo.IsMultiKey || !grokSubscriptionCredentialNeedsRefresh(strings.TrimSpace(ch.Key), now) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, _, err := RefreshGrokSubscriptionChannelCredential(ctx, ch.Id, false)
		cancel()
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("grok subscription credential refresh: channel_id=%d name=%s failed: %v", ch.Id, ch.Name, err))
			continue
		}
		refreshed++
	}
	if refreshed > 0 {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.LogWarn(context.Background(), fmt.Sprintf("grok subscription credential refresh: InitChannelCache panic: %v", recovered))
				}
			}()
			model.InitChannelCache()
		}()
		ResetProxyClientCache()
	}
}
