package preview

import "context"

type TaskRepository[T any] interface {
	GetTask(ctx context.Context, taskID string) (*T, error)
}

type TaskPreviewServiceConfig[T any, P any] struct {
	Repository      TaskRepository[T]
	BuildPreview    func(context.Context, *T, string) (*P, error)
	FinalizePreview func(context.Context, *T, *P) error
}

type TaskPreviewService[T any, P any] struct {
	repo            TaskRepository[T]
	buildPreview    func(context.Context, *T, string) (*P, error)
	finalizePreview func(context.Context, *T, *P) error
}

func NewTaskPreviewService[T any, P any](config TaskPreviewServiceConfig[T, P]) *TaskPreviewService[T, P] {
	return &TaskPreviewService[T, P]{
		repo:            config.Repository,
		buildPreview:    config.BuildPreview,
		finalizePreview: config.FinalizePreview,
	}
}

func (s *TaskPreviewService[T, P]) GetTaskPreview(ctx context.Context, taskID string, platform string) (*P, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	preview, err := s.buildPreview(ctx, task, platform)
	if err != nil {
		return nil, err
	}
	if s.finalizePreview == nil {
		return preview, nil
	}
	if err := s.finalizePreview(ctx, task, preview); err != nil {
		return nil, err
	}
	return preview, nil
}
