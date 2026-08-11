package metricstore

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testMetricKey = "test.account.usage_percent"

var registerTestMetricOnce sync.Once

func floatPointer(value float64) *float64 {
	return &value
}

func setupMetricStoreTest(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MetricSeries{}, &model.MetricPoint{}))
	model.DB = db

	registerTestMetricOnce.Do(func() {
		require.NoError(t, Register(Descriptor{
			Key:               testMetricKey,
			Unit:              "percent",
			Bucket:            5 * time.Minute,
			AllowedSubjects:   []string{"account"},
			RequiredLabels:    []string{"window"},
			MinValue:          floatPointer(0),
			MaxValue:          floatPointer(100),
			MaxSeriesPerScope: 4,
		}))
	})

	t.Cleanup(func() {
		model.DB = originalDB
	})
}

func TestRecordObservationPreservesExplicitZeroAndCanonicalizesLabels(t *testing.T) {
	setupMetricStoreTest(t)

	observedAt := time.Unix(1_700_000_123, 456_000_000)
	result, err := RecordObservation(context.Background(), Observation{
		Subject:    Subject{Type: "account", ID: "account-1"},
		ObservedAt: observedAt,
		Measurements: []Measurement{
			{MetricKey: testMetricKey, Labels: map[string]string{"window": "5h"}, Value: 0},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Inserted)

	series, err := QuerySeries(context.Background(), Query{
		MetricKey: testMetricKey,
		Subject:   Subject{Type: "account", ID: "account-1"},
		StartTime: observedAt.Add(-time.Hour),
		EndTime:   observedAt.Add(time.Hour),
		MaxSeries: 10,
	})
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, map[string]string{"window": "5h"}, series[0].Labels)
	require.Len(t, series[0].Points, 1)
	assert.Equal(t, float64(0), series[0].Points[0].Value)
	assert.Equal(t, observedAt.UnixMilli(), series[0].Points[0].ObservedAtMs)
}

func TestRecordObservationKeepsNewestObservationWithinBucket(t *testing.T) {
	setupMetricStoreTest(t)

	base := time.Unix(1_700_000_100, 0)
	write := func(observedAt time.Time, value float64) WriteResult {
		result, err := RecordObservation(context.Background(), Observation{
			Subject:    Subject{Type: "account", ID: "account-2"},
			ObservedAt: observedAt,
			Measurements: []Measurement{
				{MetricKey: testMetricKey, Labels: map[string]string{"window": "weekly"}, Value: value},
			},
		})
		require.NoError(t, err)
		return result
	}

	assert.Equal(t, 1, write(base.Add(2*time.Minute), 80).Inserted)
	assert.Equal(t, 1, write(base.Add(3*time.Minute), 20).Updated)
	assert.Equal(t, 1, write(base.Add(time.Minute), 95).Stale)

	series, err := QuerySeries(context.Background(), Query{
		MetricKey: testMetricKey,
		Subject:   Subject{Type: "account", ID: "account-2"},
		StartTime: base.Add(-time.Hour),
		EndTime:   base.Add(time.Hour),
		MaxSeries: 10,
	})
	require.NoError(t, err)
	require.Len(t, series, 1)
	require.Len(t, series[0].Points, 1)
	assert.Equal(t, 20.0, series[0].Points[0].Value)
}

func TestRecordObservationRejectsWholeBatchWhenMeasurementIsInvalid(t *testing.T) {
	setupMetricStoreTest(t)

	_, err := RecordObservation(context.Background(), Observation{
		Subject:    Subject{Type: "account", ID: "account-3"},
		ObservedAt: time.Unix(1_700_000_000, 0),
		Measurements: []Measurement{
			{MetricKey: testMetricKey, Labels: map[string]string{"window": "5h"}, Value: 25},
			{MetricKey: testMetricKey, Labels: map[string]string{"unexpected": "value"}, Value: 40},
		},
	})
	require.Error(t, err)

	var pointCount int64
	require.NoError(t, model.DB.Model(&model.MetricPoint{}).Count(&pointCount).Error)
	assert.Zero(t, pointCount)
}
