package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/pkg/metricstore"
	"github.com/QuantumNous/new-api/setting/codex_usage_setting"
)

type CodexUsageHistory struct {
	Collection   CodexUsageHistoryCollection `json:"collection"`
	From         int64                       `json:"from"`
	To           int64                       `json:"to"`
	Resolution   int64                       `json:"resolution_seconds"`
	Limits       []CodexUsageHistoryLimit    `json:"limits"`
	ResetCredits []CodexResetCreditPoint     `json:"reset_credits"`
}

type CodexUsageHistoryCollection struct {
	Enabled         bool  `json:"enabled"`
	IntervalSeconds int64 `json:"interval_seconds"`
	LastObservedAt  int64 `json:"last_observed_at,omitempty"`
	Stale           bool  `json:"stale"`
}

type CodexUsageHistoryLimit struct {
	LimitType     string                   `json:"limit_type"`
	LimitKey      string                   `json:"limit_key"`
	WindowKey     string                   `json:"window_key"`
	WindowSeconds *int64                   `json:"window_seconds"`
	Points        []CodexUsageHistoryPoint `json:"points"`
}

type CodexUsageHistoryPoint struct {
	Ts             int64    `json:"ts"`
	UsedPercent    *float64 `json:"used_percent"`
	MinUsedPercent *float64 `json:"min_used_percent"`
	MaxUsedPercent *float64 `json:"max_used_percent"`
	SampleCount    int      `json:"sample_count"`
}

type CodexResetCreditPoint struct {
	Ts          int64    `json:"ts"`
	Value       *float64 `json:"value"`
	SampleCount int      `json:"sample_count"`
}

func ResolveCodexUsageHistoryRange(rangeName string, now time.Time) (time.Time, time.Time, time.Duration, error) {
	if now.IsZero() {
		now = time.Now()
	}
	switch strings.TrimSpace(rangeName) {
	case "", "7d":
		return now.Add(-7 * 24 * time.Hour), now, time.Hour, nil
	case "24h":
		return now.Add(-24 * time.Hour), now, 15 * time.Minute, nil
	case "30d":
		return now.Add(-30 * 24 * time.Hour), now, 6 * time.Hour, nil
	case "90d":
		return now.Add(-90 * 24 * time.Hour), now, 24 * time.Hour, nil
	default:
		return time.Time{}, time.Time{}, 0, errors.New("unsupported codex usage history range")
	}
}

func QueryCodexUsageHistory(ctx context.Context, accountID string, from time.Time, to time.Time, resolution time.Duration) (CodexUsageHistory, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return CodexUsageHistory{}, errors.New("codex account id is required")
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return CodexUsageHistory{}, errors.New("codex usage history requires a valid time range")
	}
	if resolution < 5*time.Minute || resolution%time.Minute != 0 {
		return CodexUsageHistory{}, errors.New("codex usage history resolution is invalid")
	}

	subject := metricstore.Subject{Type: codexMetricSubjectType, ID: CodexAccountFingerprint(accountID)}
	usageSeries, err := metricstore.QuerySeries(ctx, metricstore.Query{
		MetricKey: CodexUsagePercentMetric,
		Subject:   subject,
		StartTime: from,
		EndTime:   to,
		MaxSeries: maxCodexAdditionalLimits,
	})
	if err != nil {
		return CodexUsageHistory{}, err
	}
	creditSeries, err := metricstore.QuerySeries(ctx, metricstore.Query{
		MetricKey: CodexResetCreditsMetric,
		Subject:   subject,
		StartTime: from,
		EndTime:   to,
		MaxSeries: 1,
	})
	if err != nil {
		return CodexUsageHistory{}, err
	}

	limits := make([]CodexUsageHistoryLimit, 0, len(usageSeries))
	lastObservedAtMs := int64(0)
	for _, series := range usageSeries {
		limitType := series.Labels["limit_type"]
		limitKey := series.Labels["limit_key"]
		windowKey := series.Labels["window_key"]
		if limitType == "" || limitKey == "" || windowKey == "" {
			continue
		}
		points, latest := downsampleCodexUsagePoints(series.Points, from, to, resolution)
		if latest == 0 {
			continue
		}
		if latest > lastObservedAtMs {
			lastObservedAtMs = latest
		}
		limits = append(limits, CodexUsageHistoryLimit{
			LimitType:     limitType,
			LimitKey:      limitKey,
			WindowKey:     windowKey,
			WindowSeconds: parseCodexWindowSeconds(windowKey),
			Points:        points,
		})
	}
	sort.Slice(limits, func(i, j int) bool {
		if limits[i].LimitType != limits[j].LimitType {
			return limits[i].LimitType == "base"
		}
		if limits[i].LimitKey != limits[j].LimitKey {
			return limits[i].LimitKey < limits[j].LimitKey
		}
		left := int64(0)
		right := int64(0)
		if limits[i].WindowSeconds != nil {
			left = *limits[i].WindowSeconds
		}
		if limits[j].WindowSeconds != nil {
			right = *limits[j].WindowSeconds
		}
		return left < right
	})

	resetCredits := []CodexResetCreditPoint{}
	if len(creditSeries) > 0 && len(creditSeries[0].Points) > 0 {
		var latest int64
		resetCredits, latest = downsampleCodexResetCreditPoints(creditSeries[0].Points, from, to, resolution)
		if latest > lastObservedAtMs {
			lastObservedAtMs = latest
		}
	}

	setting := codex_usage_setting.GetSetting()
	interval := codex_usage_setting.CollectionInterval()
	staleAfter := 2 * interval
	if staleAfter < 30*time.Minute {
		staleAfter = 30 * time.Minute
	}
	lastObservedAt := int64(0)
	stale := false
	if lastObservedAtMs > 0 {
		lastObservedAt = lastObservedAtMs / 1000
		stale = time.Since(time.UnixMilli(lastObservedAtMs)) > staleAfter
	}

	return CodexUsageHistory{
		Collection: CodexUsageHistoryCollection{
			Enabled:         setting.Enabled,
			IntervalSeconds: int64(interval / time.Second),
			LastObservedAt:  lastObservedAt,
			Stale:           stale,
		},
		From:         from.Unix(),
		To:           to.Unix(),
		Resolution:   int64(resolution / time.Second),
		Limits:       limits,
		ResetCredits: resetCredits,
	}, nil
}

type codexGaugeBucket struct {
	lastValue      float64
	lastObservedAt int64
	minValue       float64
	maxValue       float64
	sampleCount    int
}

func downsampleCodexUsagePoints(points []metricstore.Point, from time.Time, to time.Time, resolution time.Duration) ([]CodexUsageHistoryPoint, int64) {
	buckets, latest := aggregateCodexGaugePoints(points, resolution)
	resolutionSeconds := int64(resolution / time.Second)
	firstBucket := alignMetricBucket(from.Unix(), resolutionSeconds)
	result := make([]CodexUsageHistoryPoint, 0, int(to.Sub(from)/resolution)+1)
	for ts := firstBucket; ts < to.Unix(); ts += resolutionSeconds {
		bucket, ok := buckets[ts]
		if !ok {
			result = append(result, CodexUsageHistoryPoint{Ts: ts})
			continue
		}
		lastValue := bucket.lastValue
		minValue := bucket.minValue
		maxValue := bucket.maxValue
		result = append(result, CodexUsageHistoryPoint{
			Ts:             ts,
			UsedPercent:    &lastValue,
			MinUsedPercent: &minValue,
			MaxUsedPercent: &maxValue,
			SampleCount:    bucket.sampleCount,
		})
	}
	return result, latest
}

func downsampleCodexResetCreditPoints(points []metricstore.Point, from time.Time, to time.Time, resolution time.Duration) ([]CodexResetCreditPoint, int64) {
	buckets, latest := aggregateCodexGaugePoints(points, resolution)
	resolutionSeconds := int64(resolution / time.Second)
	firstBucket := alignMetricBucket(from.Unix(), resolutionSeconds)
	result := make([]CodexResetCreditPoint, 0, int(to.Sub(from)/resolution)+1)
	for ts := firstBucket; ts < to.Unix(); ts += resolutionSeconds {
		bucket, ok := buckets[ts]
		if !ok {
			result = append(result, CodexResetCreditPoint{Ts: ts})
			continue
		}
		value := bucket.lastValue
		result = append(result, CodexResetCreditPoint{Ts: ts, Value: &value, SampleCount: bucket.sampleCount})
	}
	return result, latest
}

func aggregateCodexGaugePoints(points []metricstore.Point, resolution time.Duration) (map[int64]codexGaugeBucket, int64) {
	resolutionSeconds := int64(resolution / time.Second)
	buckets := make(map[int64]codexGaugeBucket)
	latest := int64(0)
	for _, point := range points {
		bucketTs := alignMetricBucket(point.BucketTs, resolutionSeconds)
		bucket, exists := buckets[bucketTs]
		if !exists {
			bucket = codexGaugeBucket{
				lastValue:      point.Value,
				lastObservedAt: point.ObservedAtMs,
				minValue:       point.Value,
				maxValue:       point.Value,
			}
		} else {
			if point.Value < bucket.minValue {
				bucket.minValue = point.Value
			}
			if point.Value > bucket.maxValue {
				bucket.maxValue = point.Value
			}
			if point.ObservedAtMs > bucket.lastObservedAt {
				bucket.lastValue = point.Value
				bucket.lastObservedAt = point.ObservedAtMs
			}
		}
		bucket.sampleCount++
		buckets[bucketTs] = bucket
		if point.ObservedAtMs > latest {
			latest = point.ObservedAtMs
		}
	}
	return buckets, latest
}

func alignMetricBucket(timestamp int64, resolutionSeconds int64) int64 {
	return timestamp - timestamp%resolutionSeconds
}

func parseCodexWindowSeconds(windowKey string) *int64 {
	const prefix = "duration:"
	if !strings.HasPrefix(windowKey, prefix) {
		return nil
	}
	seconds, err := strconv.ParseInt(strings.TrimPrefix(windowKey, prefix), 10, 64)
	if err != nil || seconds <= 0 {
		return nil
	}
	return &seconds
}
