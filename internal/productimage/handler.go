package productimage

import (
	"context"

	"github.com/gin-gonic/gin"
)

type HandlerService interface {
	CreateProcessTask(ctx context.Context, req *ImageProcessRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error)
}

type Handler interface {
	ProcessImages(c *gin.Context)
	GetTaskResult(c *gin.Context)
	ReviewTask(c *gin.Context)
}
