package codex_usage_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOptionEnforcesCollectionAndRetentionBounds(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantError bool
	}{
		{name: "minimum interval", key: "collection_interval_minutes", value: "5"},
		{name: "interval below minimum", key: "collection_interval_minutes", value: "4", wantError: true},
		{name: "maximum retention", key: "retention_days", value: "3650"},
		{name: "negative retention", key: "retention_days", value: "-1", wantError: true},
		{name: "enabled boolean", key: "enabled", value: "true"},
		{name: "invalid boolean", key: "enabled", value: "yes", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateOption(test.key, test.value)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
