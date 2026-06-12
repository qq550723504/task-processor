package productimage

import "context"

type HandlerService interface {
	CreateProcessTask(ctx context.Context, req *ImageProcessRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error)
}
