package bootstrap

import (
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
)

func BuildConsumerProcessorCreators() consumer.ProcessorCreators {
	creators := consumer.ProcessorCreators{}
	for _, module := range platformProcessorModules() {
		creators = module.assignConsumer(creators, module.temuCreator, module.sheinCreator)
	}
	return creators
}

// BuildProcessorDependencies keeps runner-side processor wiring in bootstrap.
func BuildProcessorDependencies() runner.ProcessorDependencies {
	deps := runner.ProcessorDependencies{}
	for _, module := range platformProcessorModules() {
		deps = module.assignRunner(deps, runner.TemuProcessorCreator(module.temuCreator), runner.SheinProcessorCreator(module.sheinCreator))
	}
	return deps
}
