package productenrich

import "context"

import "github.com/gin-gonic/gin"

// ProductHandlerService is the service contract required by the HTTP handler layer.
type ProductHandlerService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
}

// ProductHandler defines the HTTP endpoints exposed by productenrich.
type ProductHandler interface {
	GenerateProduct(c *gin.Context)
	GetTaskResult(c *gin.Context)
}
