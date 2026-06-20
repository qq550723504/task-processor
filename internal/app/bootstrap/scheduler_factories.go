package bootstrap

import (
	bootstrapschedulers "task-processor/internal/app/bootstrap/schedulers"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
)

// BuildSchedulerDependencies 将平台任务工厂创建职责上提到 bootstrap 层。
func BuildSchedulerDependencies(
	schedulerRuntime consumer.SchedulerFactoryRuntime,
	cfg *config.Config,
	crawlSource runner.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies {
	return bootstrapschedulers.BuildDependencies(schedulerRuntime, cfg, crawlSource, rabbitmqClient)
}
