package module

import (
	"context"

	"task-processor/internal/core/config"
)

type Module interface {
	Name() string
	Enabled(cfg *config.Config) bool
	Register(reg *Registry) error
}

type TaskHandler interface {
	TaskType() string
	Validate(ctx context.Context, input any) error
	Execute(ctx context.Context, task any) (any, error)
}

type WorkflowHandler interface {
	WorkflowName() string
	RegisterWorkflow(reg *WorkflowRegistry) error
}
