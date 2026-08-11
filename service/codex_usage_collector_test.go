package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCollectCodexUsageDeduplicatesChannelsByAccount(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.MetricSeries{}, &model.MetricPoint{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		assert.Equal(t, "/backend-api/wham/usage", r.URL.Path)
		assert.Equal(t, "account-shared", r.Header.Get("chatgpt-account-id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":25,"limit_window_seconds":18000}}}`))
	}))
	defer upstream.Close()

	keyBytes, err := common.Marshal(map[string]string{
		"access_token": "token",
		"account_id":   "account-shared",
	})
	require.NoError(t, err)
	for id := 1; id <= 2; id++ {
		baseURL := upstream.URL
		require.NoError(t, model.DB.Create(&model.Channel{
			Id:      id,
			Type:    constant.ChannelTypeCodex,
			Key:     string(keyBytes),
			Status:  common.ChannelStatusEnabled,
			Name:    "codex",
			BaseURL: &baseURL,
		}).Error)
	}

	summary, err := CollectCodexUsage(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.TotalAccounts)
	assert.Equal(t, 1, summary.CollectedAccounts)
	assert.Zero(t, summary.FailedAccounts)
	assert.Equal(t, int32(1), requests.Load())

	var pointCount int64
	require.NoError(t, model.DB.Model(&model.MetricPoint{}).Count(&pointCount).Error)
	assert.Equal(t, int64(1), pointCount)
}
