package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestBuildPlatformConfig_LegacySyncFallsBackToProductSync(t *testing.T) {
	v := viper.New()
	v.Set("platforms.temu.sync.enabled", true)
	v.Set("platforms.temu.sync.interval", 600)

	cfg := BuildPlatformConfig(v, "platforms.temu")

	assert.True(t, cfg.ProductSync.Enabled)
	assert.Equal(t, 600, cfg.ProductSync.Interval)
}

func TestValidatePlatformConfig_RejectsConflictingLegacySync(t *testing.T) {
	cfg := PlatformConfig{
		ProductSync: ScheduledTaskConfig{
			Enabled:  true,
			Interval: 300,
		},
		SyncProduct: SyncProductConfig{
			Enabled:  true,
			Interval: 600,
		},
	}

	errors := ValidatePlatformConfig("temu", &cfg)
	assert.NotEmpty(t, errors)

	var joined []string
	for _, err := range errors {
		joined = append(joined, err.Error())
	}
	assert.Contains(t, strings.Join(joined, "\n"), "sync and productSync intervals conflict")
}

func TestValidatePlatformConfig_FlagsDeprecatedLegacySyncFields(t *testing.T) {
	cfg := PlatformConfig{
		SyncProduct: SyncProductConfig{
			Enabled:   true,
			Interval:  300,
			BatchSize: 10,
			StoreIDs:  []int64{1},
		},
	}

	errors := ValidatePlatformConfig("shein", &cfg)
	assert.Len(t, errors, 2)
}
