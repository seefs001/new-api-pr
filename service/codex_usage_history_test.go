package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/metricstore"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQueryCodexUsageHistoryDownsamplesGaugeAndPreservesGaps(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MetricSeries{}, &model.MetricPoint{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	start := time.Unix(1_700_000_000, 0).Truncate(time.Hour)
	accountID := "history-account"
	write := func(offset time.Duration, value float64) {
		observation, normalizeErr := NormalizeCodexUsage(CodexUsagePayload{
			RateLimit: &CodexRateLimit{
				PrimaryWindow: &CodexRateLimitWindow{
					UsedPercent:        &value,
					LimitWindowSeconds: int64Pointer(5 * 60 * 60),
				},
			},
		}, accountID, start.Add(offset))
		require.NoError(t, normalizeErr)
		_, recordErr := metricstore.RecordObservation(context.Background(), observation)
		require.NoError(t, recordErr)
	}

	write(5*time.Minute, 10)
	write(35*time.Minute, 40)
	write(2*time.Hour+10*time.Minute, 70)

	history, err := QueryCodexUsageHistory(
		context.Background(),
		accountID,
		start,
		start.Add(3*time.Hour),
		time.Hour,
	)
	require.NoError(t, err)
	require.Len(t, history.Limits, 1)
	require.Len(t, history.Limits[0].Points, 3)

	first := history.Limits[0].Points[0]
	require.NotNil(t, first.UsedPercent)
	require.NotNil(t, first.MinUsedPercent)
	require.NotNil(t, first.MaxUsedPercent)
	assert.Equal(t, 40.0, *first.UsedPercent)
	assert.Equal(t, 10.0, *first.MinUsedPercent)
	assert.Equal(t, 40.0, *first.MaxUsedPercent)
	assert.Equal(t, 2, first.SampleCount)

	assert.Nil(t, history.Limits[0].Points[1].UsedPercent)
	assert.Zero(t, history.Limits[0].Points[1].SampleCount)
	require.NotNil(t, history.Limits[0].Points[2].UsedPercent)
	assert.Equal(t, 70.0, *history.Limits[0].Points[2].UsedPercent)
}

func TestQueryCodexUsageHistoryOmitsSeriesWithoutSamplesInRange(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MetricSeries{}, &model.MetricPoint{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	observedAt := time.Unix(1_700_000_000, 0)
	value := 25.0
	observation, err := NormalizeCodexUsage(CodexUsagePayload{
		RateLimit: &CodexRateLimit{
			PrimaryWindow: &CodexRateLimitWindow{
				UsedPercent:        &value,
				LimitWindowSeconds: int64Pointer(5 * 60 * 60),
			},
		},
	}, "history-account", observedAt)
	require.NoError(t, err)
	_, err = metricstore.RecordObservation(context.Background(), observation)
	require.NoError(t, err)

	history, err := QueryCodexUsageHistory(
		context.Background(),
		"history-account",
		observedAt.Add(24*time.Hour),
		observedAt.Add(25*time.Hour),
		time.Hour,
	)
	require.NoError(t, err)
	assert.Empty(t, history.Limits)
}

func int64Pointer(value int64) *int64 {
	return &value
}
