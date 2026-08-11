package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCodexUsageBuildsStableBaseAndAdditionalSeries(t *testing.T) {
	usedZero := 0.0
	usedFiveHour := 37.5
	usedWeekly := 81.25
	fiveHours := int64(5 * 60 * 60)
	oneWeek := int64(7 * 24 * 60 * 60)
	credits := 2

	payload := CodexUsagePayload{
		RateLimit: &CodexRateLimit{
			PrimaryWindow:   &CodexRateLimitWindow{UsedPercent: &usedFiveHour, LimitWindowSeconds: &fiveHours},
			SecondaryWindow: &CodexRateLimitWindow{UsedPercent: &usedWeekly, LimitWindowSeconds: &oneWeek},
		},
		AdditionalRateLimits: []CodexAdditionalRateLimit{
			{
				MeteredFeature: "codex_feature",
				RateLimit: &CodexRateLimit{
					PrimaryWindow: &CodexRateLimitWindow{UsedPercent: &usedZero, LimitWindowSeconds: &fiveHours},
				},
			},
		},
		RateLimitResetCredits: &CodexRateLimitResetCredits{AvailableCount: &credits},
	}

	observedAt := time.Unix(1_700_000_000, 0)
	observation, err := NormalizeCodexUsage(payload, "account-123", observedAt)
	require.NoError(t, err)
	assert.Equal(t, CodexAccountFingerprint("account-123"), observation.Subject.ID)
	assert.Equal(t, observedAt, observation.ObservedAt)
	require.Len(t, observation.Measurements, 4)

	assert.Equal(t, CodexUsagePercentMetric, observation.Measurements[0].MetricKey)
	assert.Equal(t, map[string]string{
		"limit_type": "base",
		"limit_key":  "base",
		"window_key": "duration:18000",
	}, observation.Measurements[0].Labels)
	assert.Equal(t, 37.5, observation.Measurements[0].Value)

	assert.Equal(t, map[string]string{
		"limit_type": "additional",
		"limit_key":  "codex_feature",
		"window_key": "duration:18000",
	}, observation.Measurements[2].Labels)
	assert.Zero(t, observation.Measurements[2].Value)
	assert.Equal(t, CodexResetCreditsMetric, observation.Measurements[3].MetricKey)
	assert.Equal(t, 2.0, observation.Measurements[3].Value)
}

func TestNormalizeCodexUsageUsesDirectAdditionalWindowsAndSlotFallback(t *testing.T) {
	used := 45.0
	payload := CodexUsagePayload{
		AdditionalRateLimits: []CodexAdditionalRateLimit{
			{
				LimitName:     "  Priority   Models ",
				PrimaryWindow: &CodexRateLimitWindow{UsedPercent: &used},
			},
		},
	}

	observation, err := NormalizeCodexUsage(payload, "account-456", time.Now())
	require.NoError(t, err)
	require.Len(t, observation.Measurements, 1)
	assert.Equal(t, map[string]string{
		"limit_type": "additional",
		"limit_key":  "priority_models",
		"window_key": "slot:primary",
	}, observation.Measurements[0].Labels)
}

func TestNormalizeCodexUsageRejectsPayloadWithoutValidMeasurements(t *testing.T) {
	invalid := 101.0
	payload := CodexUsagePayload{
		RateLimit: &CodexRateLimit{
			PrimaryWindow: &CodexRateLimitWindow{UsedPercent: &invalid},
		},
	}

	_, err := NormalizeCodexUsage(payload, "account-789", time.Now())
	require.Error(t, err)
}
