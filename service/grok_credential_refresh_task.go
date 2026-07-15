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
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	grokCredentialRefreshTickInterval = 10 * time.Minute
	grokCredentialRefreshThreshold    = time.Hour
	grokCredentialRefreshTimeout      = 20 * time.Second
)

var (
	grokCredentialRefreshOnce    sync.Once
	grokCredentialRefreshRunning atomic.Bool
)

func StartGrokCredentialAutoRefreshTask() {
	grokCredentialRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("grok credential auto-refresh task started: tick=%s threshold=%s", grokCredentialRefreshTickInterval, grokCredentialRefreshThreshold))
			ticker := time.NewTicker(grokCredentialRefreshTickInterval)
			defer ticker.Stop()

			runGrokCredentialAutoRefreshOnce()
			for range ticker.C {
				runGrokCredentialAutoRefreshOnce()
			}
		})
	})
}

func runGrokCredentialAutoRefreshOnce() {
	if !grokCredentialRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer grokCredentialRefreshRunning.Store(false)

	ctx := context.Background()
	now := time.Now()
	var channels []*model.Channel
	err := model.DB.
		Select("id", "name", "key", "status", "channel_info").
		Where("type = ? AND (status = ? OR status = ?)",
			constant.ChannelTypeGrok,
			common.ChannelStatusEnabled,
			common.ChannelStatusAutoDisabled,
		).
		Order("id asc").
		Find(&channels).Error
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("grok credential auto-refresh: query channels failed: %v", err))
		return
	}

	refreshed := 0
	for _, ch := range channels {
		if ch == nil || ch.ChannelInfo.IsMultiKey {
			continue
		}
		credential, err := dto.ParseGrokCredential(strings.TrimSpace(ch.Key))
		if err != nil || strings.TrimSpace(credential.RefreshToken) == "" {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(credential.Expired))
		if err == nil && expiresAt.Sub(now) > grokCredentialRefreshThreshold {
			continue
		}

		refreshCtx, cancel := context.WithTimeout(ctx, grokCredentialRefreshTimeout)
		newCredential, _, err := RefreshGrokChannelCredential(refreshCtx, ch.Id, GrokCredentialRefreshOptions{})
		cancel()
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("grok credential auto-refresh: channel_id=%d name=%s refresh failed: %v", ch.Id, ch.Name, err))
			continue
		}
		refreshed++
		logger.LogInfo(ctx, fmt.Sprintf("grok credential auto-refresh: channel_id=%d name=%s refreshed, expires_at=%s", ch.Id, ch.Name, newCredential.Expired))
	}

	if refreshed > 0 {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.LogWarn(ctx, fmt.Sprintf("grok credential auto-refresh: InitChannelCache panic: %v", recovered))
				}
			}()
			model.InitChannelCache()
		}()
		ResetProxyClientCache()
	}
}
