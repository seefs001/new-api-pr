package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/metricstore"
	"github.com/QuantumNous/new-api/setting/codex_usage_setting"
)

const (
	CodexUsagePercentMetric  = "codex.usage.used_percent"
	CodexResetCreditsMetric  = "codex.reset_credits.available"
	codexMetricSubjectType   = "codex_account"
	maxCodexAdditionalLimits = 128
)

var (
	codexUsageMinPercent = 0.0
	codexUsageMaxPercent = 100.0
	codexResetCreditMin  = 0.0
)

func init() {
	metricstore.MustRegister(metricstore.Descriptor{
		Key:               CodexUsagePercentMetric,
		Unit:              "percent",
		Bucket:            5 * time.Minute,
		AllowedSubjects:   []string{codexMetricSubjectType},
		RequiredLabels:    []string{"limit_type", "limit_key", "window_key"},
		MinValue:          &codexUsageMinPercent,
		MaxValue:          &codexUsageMaxPercent,
		MaxSeriesPerScope: maxCodexAdditionalLimits,
	})
	metricstore.MustRegister(metricstore.Descriptor{
		Key:               CodexResetCreditsMetric,
		Unit:              "count",
		Bucket:            5 * time.Minute,
		AllowedSubjects:   []string{codexMetricSubjectType},
		MinValue:          &codexResetCreditMin,
		MaxSeriesPerScope: 1,
	})
}

type CodexRateLimitWindow struct {
	UsedPercent        *float64 `json:"used_percent,omitempty"`
	ResetAt            *int64   `json:"reset_at,omitempty"`
	ResetAfterSeconds  *int64   `json:"reset_after_seconds,omitempty"`
	LimitWindowSeconds *int64   `json:"limit_window_seconds,omitempty"`
}

type CodexRateLimit struct {
	PlanType        string                `json:"plan_type,omitempty"`
	Allowed         *bool                 `json:"allowed,omitempty"`
	LimitReached    *bool                 `json:"limit_reached,omitempty"`
	PrimaryWindow   *CodexRateLimitWindow `json:"primary_window,omitempty"`
	SecondaryWindow *CodexRateLimitWindow `json:"secondary_window,omitempty"`
}

type CodexAdditionalRateLimit struct {
	LimitName       string                `json:"limit_name,omitempty"`
	MeteredFeature  string                `json:"metered_feature,omitempty"`
	RateLimit       *CodexRateLimit       `json:"rate_limit,omitempty"`
	PrimaryWindow   *CodexRateLimitWindow `json:"primary_window,omitempty"`
	SecondaryWindow *CodexRateLimitWindow `json:"secondary_window,omitempty"`
	PlanType        string                `json:"plan_type,omitempty"`
}

type CodexRateLimitResetCredits struct {
	AvailableCount *int `json:"available_count,omitempty"`
}

type CodexUsagePayload struct {
	PlanType              string                      `json:"plan_type,omitempty"`
	UserID                string                      `json:"user_id,omitempty"`
	Email                 string                      `json:"email,omitempty"`
	RateLimit             *CodexRateLimit             `json:"rate_limit,omitempty"`
	AdditionalRateLimits  []CodexAdditionalRateLimit  `json:"additional_rate_limits,omitempty"`
	RateLimitResetCredits *CodexRateLimitResetCredits `json:"rate_limit_reset_credits,omitempty"`
}

func DecodeCodexUsagePayload(body []byte) (CodexUsagePayload, error) {
	var payload CodexUsagePayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return CodexUsagePayload{}, fmt.Errorf("decode codex usage payload: %w", err)
	}
	return payload, nil
}

func CodexAccountFingerprint(accountID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(accountID)))
	return hex.EncodeToString(digest[:])
}

func RecordCodexUsageIfEnabled(ctx context.Context, body []byte, accountID string, observedAt time.Time) (bool, error) {
	if !codex_usage_setting.GetSetting().Enabled {
		return false, nil
	}
	payload, err := DecodeCodexUsagePayload(body)
	if err != nil {
		return false, err
	}
	observation, err := NormalizeCodexUsage(payload, accountID, observedAt)
	if err != nil {
		return false, err
	}
	_, err = metricstore.RecordObservation(ctx, observation)
	return err == nil, err
}

func NormalizeCodexUsage(payload CodexUsagePayload, accountID string, observedAt time.Time) (metricstore.Observation, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return metricstore.Observation{}, errors.New("codex account id is required")
	}
	if observedAt.IsZero() {
		return metricstore.Observation{}, errors.New("codex usage observation time is required")
	}

	measurements := make([]metricstore.Measurement, 0, 5+len(payload.AdditionalRateLimits)*2)
	identities := map[string]struct{}{}
	appendWindow := func(limitType string, limitKey string, slot string, window *CodexRateLimitWindow) {
		measurement, ok := codexWindowMeasurement(limitType, limitKey, slot, window)
		if !ok {
			return
		}
		identity := measurement.MetricKey + "\x00" + measurement.Labels["limit_type"] + "\x00" + measurement.Labels["limit_key"] + "\x00" + measurement.Labels["window_key"]
		if _, exists := identities[identity]; exists {
			return
		}
		identities[identity] = struct{}{}
		measurements = append(measurements, measurement)
	}

	if payload.RateLimit != nil {
		appendWindow("base", "base", "primary", payload.RateLimit.PrimaryWindow)
		appendWindow("base", "base", "secondary", payload.RateLimit.SecondaryWindow)
	}

	additionalLimits := payload.AdditionalRateLimits
	if len(additionalLimits) > maxCodexAdditionalLimits {
		additionalLimits = additionalLimits[:maxCodexAdditionalLimits]
	}
	for _, additional := range additionalLimits {
		limitKey := normalizeCodexLimitKey(additional.MeteredFeature)
		if limitKey == "" {
			limitKey = normalizeCodexLimitKey(additional.LimitName)
		}
		if limitKey == "" {
			continue
		}

		primary := additional.PrimaryWindow
		secondary := additional.SecondaryWindow
		if additional.RateLimit != nil && (additional.RateLimit.PrimaryWindow != nil || additional.RateLimit.SecondaryWindow != nil) {
			primary = additional.RateLimit.PrimaryWindow
			secondary = additional.RateLimit.SecondaryWindow
		}
		appendWindow("additional", limitKey, "primary", primary)
		appendWindow("additional", limitKey, "secondary", secondary)
	}

	if payload.RateLimitResetCredits != nil && payload.RateLimitResetCredits.AvailableCount != nil {
		available := *payload.RateLimitResetCredits.AvailableCount
		if available >= 0 {
			measurements = append(measurements, metricstore.Measurement{
				MetricKey: CodexResetCreditsMetric,
				Labels:    map[string]string{},
				Value:     float64(available),
			})
		}
	}

	if len(measurements) == 0 {
		return metricstore.Observation{}, errors.New("codex usage payload contains no valid measurements")
	}
	return metricstore.Observation{
		Subject: metricstore.Subject{
			Type: codexMetricSubjectType,
			ID:   CodexAccountFingerprint(accountID),
		},
		ObservedAt:   observedAt,
		Measurements: measurements,
	}, nil
}

func codexWindowMeasurement(limitType string, limitKey string, slot string, window *CodexRateLimitWindow) (metricstore.Measurement, bool) {
	if window == nil || window.UsedPercent == nil {
		return metricstore.Measurement{}, false
	}
	usedPercent := *window.UsedPercent
	if math.IsNaN(usedPercent) || math.IsInf(usedPercent, 0) || usedPercent < 0 || usedPercent > 100 {
		return metricstore.Measurement{}, false
	}

	windowKey := "slot:" + slot
	if window.LimitWindowSeconds != nil && *window.LimitWindowSeconds > 0 {
		windowKey = "duration:" + strconv.FormatInt(*window.LimitWindowSeconds, 10)
	}
	return metricstore.Measurement{
		MetricKey: CodexUsagePercentMetric,
		Labels: map[string]string{
			"limit_type": limitType,
			"limit_key":  limitKey,
			"window_key": windowKey,
		},
		Value: usedPercent,
	}, true
}

func normalizeCodexLimitKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastSeparator := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' {
			builder.WriteRune(r)
			lastSeparator = false
			continue
		}
		if !lastSeparator && builder.Len() > 0 {
			builder.WriteByte('_')
			lastSeparator = true
		}
	}
	return strings.Trim(builder.String(), "_")
}
