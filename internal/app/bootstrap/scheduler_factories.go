package bootstrap

import (
	"task-processor/internal/app/ports"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
)

// BuildSchedulerDependencies 将平台任务工厂创建职责上提到 bootstrap 层。
func BuildSchedulerDependencies(
	managementClient *management.ClientManager,
	cfg *config.Config,
	amazonProcessor ports.ProductSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies {
	deps := runner.SchedulerDependencies{}
	for _, module := range platformSchedulerModules() {
		deps = module.assign(deps, module.build(managementClient, cfg, amazonProcessor, rabbitmqClient))
	}
	return deps
}
