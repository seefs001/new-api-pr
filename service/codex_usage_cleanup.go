package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/pkg/metricstore"
	"github.com/QuantumNous/new-api/setting/codex_usage_setting"
)

type CodexUsageCleanupSummary struct {
	DeletedPoints int64 `json:"deleted_points"`
}

func CleanupCodexUsageMetrics(ctx context.Context) (CodexUsageCleanupSummary, error) {
	retentionDays := codex_usage_setting.GetSetting().RetentionDays
	if retentionDays <= 0 {
		return CodexUsageCleanupSummary{}, nil
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	usageDeleted, err := metricstore.Prune(ctx, CodexUsagePercentMetric, cutoff)
	if err != nil {
		return CodexUsageCleanupSummary{}, err
	}
	creditsDeleted, err := metricstore.Prune(ctx, CodexResetCreditsMetric, cutoff)
	if err != nil {
		return CodexUsageCleanupSummary{}, err
	}
	return CodexUsageCleanupSummary{DeletedPoints: usageDeleted + creditsDeleted}, nil
}
