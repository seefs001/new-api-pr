package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/metricstore"
)

const (
	codexUsageCollectorConcurrency   = 4
	codexUsageCollectorMaxCandidates = 3
	codexUsageCollectorTimeout       = 15 * time.Second
)

type CodexUsageCollectionSummary struct {
	TotalAccounts     int `json:"total_accounts"`
	CollectedAccounts int `json:"collected_accounts"`
	FailedAccounts    int `json:"failed_accounts"`
	SkippedChannels   int `json:"skipped_channels"`
}

type codexUsageAccountCandidates struct {
	accountID string
	channels  []*model.Channel
}

type codexUsageCollectionResult struct {
	collected bool
	err       error
}

func CollectCodexUsage(ctx context.Context, progress func(processed, total int)) (CodexUsageCollectionSummary, error) {
	var channels []*model.Channel
	err := model.DB.WithContext(ctx).
		Where("type = ? AND status IN ?", constant.ChannelTypeCodex, []int{common.ChannelStatusEnabled, common.ChannelStatusAutoDisabled}).
		Order("id ASC").
		Find(&channels).Error
	if err != nil {
		return CodexUsageCollectionSummary{}, err
	}

	groups := make(map[string]*codexUsageAccountCandidates)
	summary := CodexUsageCollectionSummary{}
	for _, channel := range channels {
		if channel == nil || channel.ChannelInfo.IsMultiKey {
			summary.SkippedChannels++
			continue
		}
		oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(channel.Key))
		if err != nil || strings.TrimSpace(oauthKey.AccountID) == "" || strings.TrimSpace(oauthKey.AccessToken) == "" {
			summary.SkippedChannels++
			continue
		}
		fingerprint := CodexAccountFingerprint(oauthKey.AccountID)
		group, ok := groups[fingerprint]
		if !ok {
			group = &codexUsageAccountCandidates{accountID: strings.TrimSpace(oauthKey.AccountID)}
			groups[fingerprint] = group
		}
		group.channels = append(group.channels, channel)
	}

	accounts := make([]*codexUsageAccountCandidates, 0, len(groups))
	fingerprints := make([]string, 0, len(groups))
	for fingerprint := range groups {
		fingerprints = append(fingerprints, fingerprint)
	}
	sort.Strings(fingerprints)
	for _, fingerprint := range fingerprints {
		group := groups[fingerprint]
		sort.SliceStable(group.channels, func(i, j int) bool {
			leftEnabled := group.channels[i].Status == common.ChannelStatusEnabled
			rightEnabled := group.channels[j].Status == common.ChannelStatusEnabled
			if leftEnabled != rightEnabled {
				return leftEnabled
			}
			return group.channels[i].Id < group.channels[j].Id
		})
		accounts = append(accounts, group)
	}
	summary.TotalAccounts = len(accounts)
	if len(accounts) == 0 {
		if progress != nil {
			progress(0, 0)
		}
		return summary, nil
	}

	workerCount := codexUsageCollectorConcurrency
	if workerCount > len(accounts) {
		workerCount = len(accounts)
	}
	jobs := make(chan *codexUsageAccountCandidates)
	results := make(chan codexUsageCollectionResult, len(accounts))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for account := range jobs {
				err := collectCodexUsageAccount(ctx, account)
				results <- codexUsageCollectionResult{collected: err == nil, err: err}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, account := range accounts {
			select {
			case <-ctx.Done():
				return
			case jobs <- account:
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	processed := 0
	var lastErr error
	for result := range results {
		processed++
		if result.collected {
			summary.CollectedAccounts++
		} else {
			summary.FailedAccounts++
			lastErr = result.err
		}
		if progress != nil {
			progress(processed, len(accounts))
		}
	}
	if err := ctx.Err(); err != nil {
		return summary, err
	}
	if summary.CollectedAccounts == 0 && summary.FailedAccounts > 0 {
		if lastErr == nil {
			lastErr = errors.New("all codex usage account collections failed")
		}
		return summary, lastErr
	}
	return summary, nil
}

func collectCodexUsageAccount(ctx context.Context, account *codexUsageAccountCandidates) error {
	candidateCount := len(account.channels)
	if candidateCount > codexUsageCollectorMaxCandidates {
		candidateCount = codexUsageCollectorMaxCandidates
	}
	var lastErr error
	for _, channel := range account.channels[:candidateCount] {
		oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(channel.Key))
		if err != nil {
			lastErr = err
			continue
		}
		client, err := GetHttpClientWithProxy(channel.GetSetting().Proxy)
		if err != nil {
			lastErr = err
			continue
		}

		observedAt := time.Now()
		requestCtx, cancel := context.WithTimeout(ctx, codexUsageCollectorTimeout)
		statusCode, body, fetchErr := FetchCodexWhamUsage(
			requestCtx,
			client,
			channel.GetBaseURL(),
			strings.TrimSpace(oauthKey.AccessToken),
			account.accountID,
		)
		cancel()
		if fetchErr != nil {
			lastErr = fetchErr
			continue
		}
		if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
			lastErr = fmt.Errorf("codex usage upstream status: %d", statusCode)
			continue
		}
		payload, err := DecodeCodexUsagePayload(body)
		if err != nil {
			lastErr = err
			continue
		}
		observation, err := NormalizeCodexUsage(payload, account.accountID, observedAt)
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := metricstore.RecordObservation(ctx, observation); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("codex usage account has no usable channel candidates")
	}
	return lastErr
}
