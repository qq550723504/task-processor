package bootstrap

import (
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
)

// BuildSchedulerDependencies 将平台任务工厂创建职责上提到 bootstrap 层。
func BuildSchedulerDependencies(
	managementClient *management.ClientManager,
	cfg *config.Config,
	crawlSource runner.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies {
	boundFetcherBuilder := platformbase.BindProductFetcherBuilder(platformbase.NewDefaultProductFetcherBuilder(), crawlSource)
	deps := runner.SchedulerDependencies{}
	for _, module := range platformSchedulerModules() {
		deps = module.assign(deps, module.build(managementClient, cfg, boundFetcherBuilder, rabbitmqClient))
	}
	return deps
}
