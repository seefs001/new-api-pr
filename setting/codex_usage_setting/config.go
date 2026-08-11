package codex_usage_setting

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	defaultCollectionIntervalMinutes = 15
	defaultRetentionDays             = 90
	minCollectionIntervalMinutes     = 5
	maxCollectionIntervalMinutes     = 24 * 60
	maxRetentionDays                 = 3650
)

type CodexUsageSetting struct {
	Enabled                   bool `json:"enabled"`
	CollectionIntervalMinutes int  `json:"collection_interval_minutes"`
	RetentionDays             int  `json:"retention_days"`
}

var codexUsageSetting = CodexUsageSetting{
	Enabled:                   false,
	CollectionIntervalMinutes: defaultCollectionIntervalMinutes,
	RetentionDays:             defaultRetentionDays,
}

func init() {
	config.GlobalConfig.Register("codex_usage_setting", &codexUsageSetting)
}

func GetSetting() CodexUsageSetting {
	setting := codexUsageSetting
	if setting.CollectionIntervalMinutes < minCollectionIntervalMinutes || setting.CollectionIntervalMinutes > maxCollectionIntervalMinutes {
		setting.CollectionIntervalMinutes = defaultCollectionIntervalMinutes
	}
	if setting.RetentionDays < 0 || setting.RetentionDays > maxRetentionDays {
		setting.RetentionDays = defaultRetentionDays
	}
	return setting
}

func CollectionInterval() time.Duration {
	return time.Duration(GetSetting().CollectionIntervalMinutes) * time.Minute
}

func ValidateOption(key string, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "enabled":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("invalid Codex usage history enabled value: %w", err)
		}
	case "collection_interval_minutes":
		minutes, err := strconv.Atoi(value)
		if err != nil || minutes < minCollectionIntervalMinutes || minutes > maxCollectionIntervalMinutes {
			return fmt.Errorf("Codex usage collection interval must be between %d and %d minutes", minCollectionIntervalMinutes, maxCollectionIntervalMinutes)
		}
	case "retention_days":
		days, err := strconv.Atoi(value)
		if err != nil || days < 0 || days > maxRetentionDays {
			return fmt.Errorf("Codex usage retention must be between 0 and %d days", maxRetentionDays)
		}
	default:
		return fmt.Errorf("unsupported Codex usage setting %q", key)
	}
	return nil
}
