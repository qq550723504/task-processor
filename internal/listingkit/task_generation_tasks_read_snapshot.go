package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskGenerationTasksReadSnapshot struct {
	task  *Task
	tasks []assetgeneration.Task
}

type taskGenerationTasksReadPagePhase struct{}

type taskGenerationTasksReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationTasksReadSnapshotPhase(service *taskGenerationService) *taskGenerationTasksReadSnapshotPhase {
	return &taskGenerationTasksReadSnapshotPhase{service: service}
}

func buildTaskGenerationTasksReadPagePhase() *taskGenerationTasksReadPagePhase {
	return &taskGenerationTasksReadPagePhase{}
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

func (p *taskGenerationTasksReadPagePhase) run(snapshot *taskGenerationTasksReadSnapshot, query *GenerationTaskQuery) *GenerationTaskPage {
	if snapshot == nil {
		snapshot = &taskGenerationTasksReadSnapshot{task: &Task{}}
	}
	if snapshot.task == nil {
		snapshot.task = &Task{}
	}
	filtered := filterGenerationTasks(snapshot.tasks, query)
	sorted := sortGenerationTasks(filtered, query)
	paged, meta := paginateGenerationTasks(sorted, query)
	return buildGenerationTaskPage(snapshot.task.ID, snapshot.task.UpdatedAt, filtered, paged, meta)
}
