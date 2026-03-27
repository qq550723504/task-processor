package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

type schedulerRuntimeModule struct {
	name    string
	enabled func(*config.Config) bool
	build   func(*schedulerServiceImpl, *config.Config) (platformTaskConfig, bool)
}

func (s *schedulerServiceImpl) schedulerModules() []schedulerRuntimeModule {
	return []schedulerRuntimeModule{
		{
			name:    "temu",
			enabled: func(cfg *config.Config) bool { return cfg.Platforms.Temu.SchedulerEnabled },
			build: func(s *schedulerServiceImpl, cfg *config.Config) (platformTaskConfig, bool) {
				creator := s.resolveTemuFactoryCreator()
				if creator == nil {
					return platformTaskConfig{}, false
				}
				return buildPlatformTaskConfig("TEMU", cfg.Platforms.Temu, func() scheduler.TaskFactory {
					return creator(cfg)
				}), true
			},
		},
		{
			name:    "shein",
			enabled: func(cfg *config.Config) bool { return cfg.Platforms.Shein.SchedulerEnabled },
			build: func(s *schedulerServiceImpl, cfg *config.Config) (platformTaskConfig, bool) {
				creator := s.resolveSheinFactoryCreator()
				if creator == nil {
					return platformTaskConfig{}, false
				}
				return buildPlatformTaskConfig("SHEIN", cfg.Platforms.Shein, func() scheduler.TaskFactory {
					return creator(cfg)
				}), true
			},
		},
	}
}
