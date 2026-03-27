package runner

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestGetPlatformConfigsSkipsModulesWithoutFactoryCreator(t *testing.T) {
	t.Parallel()

	service := &schedulerServiceImpl{}
	cfg := &config.Config{}
	cfg.Platforms.Temu.SchedulerEnabled = true
	cfg.Platforms.Shein.SchedulerEnabled = true

	configs := service.getPlatformConfigs(cfg)
	if len(configs) != 0 {
		t.Fatalf("expected no platform configs when factory creators are missing, got %d", len(configs))
	}
}
