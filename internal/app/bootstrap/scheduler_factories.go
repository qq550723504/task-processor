package bootstrap

import (
	"task-processor/internal/app/runner"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

// BuildSchedulerDependencies 将平台任务工厂创建职责上提到 bootstrap 层。
func BuildSchedulerDependencies(
	managementClient *management.ClientManager,
	cfg *config.Config,
	amazonProcessor platformbase.AmazonCrawler,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies {
	return runner.SchedulerDependencies{
		TemuFactoryCreator: func(currentCfg *config.Config) appscheduler.TaskFactory {
			effectiveCfg := currentCfg
			if effectiveCfg == nil {
				effectiveCfg = cfg
			}
			return temuscheduler.NewTemuTaskFactory(
				managementClient,
				amazonProcessor,
				&effectiveCfg.Amazon,
				&effectiveCfg.Platforms.Temu.Monitor,
				rabbitmqClient,
			)
		},
		SheinFactoryCreator: func(currentCfg *config.Config) appscheduler.TaskFactory {
			effectiveCfg := currentCfg
			if effectiveCfg == nil {
				effectiveCfg = cfg
			}
			return sheinscheduler.NewSheinTaskFactory(
				managementClient,
				amazonProcessor,
				&effectiveCfg.Amazon,
				&effectiveCfg.Platforms.Shein.Monitor,
				rabbitmqClient,
			)
		},
	}
}
