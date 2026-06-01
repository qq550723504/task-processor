package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskGenerationTasksReadSnapshot struct {
	task  *Task
	tasks []assetgeneration.Task
}

type taskGenerationTasksReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationTasksReadSnapshotPhase(service *taskGenerationService) *taskGenerationTasksReadSnapshotPhase {
	return &taskGenerationTasksReadSnapshotPhase{service: service}
}

func (p *taskGenerationTasksReadSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationTasksReadSnapshot, error) {
	if p == nil || p.service == nil {
		return &taskGenerationTasksReadSnapshot{}, nil
	}

	task, err := p.service.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := p.service.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}

	return &taskGenerationTasksReadSnapshot{
		task:  task,
		tasks: tasks,
	}, nil
}
