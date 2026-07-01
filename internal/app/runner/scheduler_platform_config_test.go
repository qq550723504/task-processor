package runner

import (
	"context"
	"testing"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/listingruntime"

	"github.com/sirupsen/logrus"
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

func TestGetPlatformConfigsIncludesPlatformWithAdminScheduledConfig(t *testing.T) {
	t.Parallel()

	service := &schedulerServiceImpl{
		logger: logrus.New(),
		storeRuntime: &stubSchedulerStoreRuntime{
			listScheduledTaskConfigsFunc: func(_ context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
				if platformName == "SHEIN" && taskType == scheduler.TaskTypeInventory {
					return []listingruntime.ScheduledTaskConfig{
						{StoreID: 962, Platform: "shein", TaskType: "inventory", Enabled: true, IntervalSeconds: 3600},
					}, nil
				}
				return nil, nil
			},
		},
		sheinFactoryCreator: func(*config.Config) scheduler.TaskFactory {
			return stubSchedulerTaskFactory{}
		},
	}
	cfg := &config.Config{}
	cfg.Platforms.Shein.SchedulerEnabled = false

	configs := service.getPlatformConfigs(cfg)
	if len(configs) != 1 {
		t.Fatalf("expected SHEIN platform config from admin scheduled config, got %d", len(configs))
	}
	if configs[0].PlatformName != "SHEIN" {
		t.Fatalf("expected SHEIN platform, got %s", configs[0].PlatformName)
	}
}

type stubSchedulerTaskFactory struct{}

func (stubSchedulerTaskFactory) CreateTask(context.Context, scheduler.TaskConfig) (scheduler.Task, error) {
	return nil, nil
}

func (stubSchedulerTaskFactory) SupportedPlatform() string { return "SHEIN" }

func (stubSchedulerTaskFactory) SupportedTaskTypes() []scheduler.TaskType {
	return []scheduler.TaskType{scheduler.TaskTypeInventory}
}
