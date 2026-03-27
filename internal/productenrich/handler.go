package productenrich

import (
	"context"

	"github.com/gin-gonic/gin"
)

// ProductHandlerService 是 HTTP handler 层需要的服务契约。
type ProductHandlerService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
}

// ProductHandler 定义 productenrich 的 HTTP 端点。
type ProductHandler interface {
	GenerateProduct(c *gin.Context)
	GetTaskResult(c *gin.Context)
}
