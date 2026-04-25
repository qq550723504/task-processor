package bootstrap

import (
	bootstrapprocessors "task-processor/internal/app/bootstrap/processors"
	"task-processor/internal/app/runner"
)

// BuildProcessorDependencies keeps runner-side processor wiring in bootstrap.
func BuildProcessorDependencies() runner.ProcessorDependencies {
	return bootstrapprocessors.BuildRunnerProcessorDependencies()
}
