// package productenrich 提供 HTTP API 处理器
package productenrich

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// ProductHandlerService HTTP handler 依赖的产品服务接口
type ProductHandlerService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
}

// ProductHandler 产品处理器接口
type ProductHandler interface {
	// GenerateProduct 处理产品生成请求
	GenerateProduct(c *gin.Context)
	// GetTaskResult 处理任务结果查询请求
	GetTaskResult(c *gin.Context)
}

// productHandler 产品处理器实现
type productHandler struct {
	productService ProductHandlerService
}

// NewProductHandler 创建新的产品处理器
func NewProductHandler(productService ProductHandlerService) (ProductHandler, error) {
	if productService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}

	return &productHandler{
		productService: productService,
	}, nil
}

// GenerateProduct 处理产品生成请求
func (h *productHandler) GenerateProduct(c *gin.Context) {
	var req GenerateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("invalid request body")
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(), // 绑定错误可以直接返回，属于客户端问题
		})
		return
	}

	task, err := h.productService.CreateGenerateTask(c.Request.Context(), &req)
	if err != nil {
		logrus.WithError(err).Error("failed to create task")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "task_creation_failed",
			Message: "任务创建失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, TaskResponse{
		TaskID:    task.ID,
		Status:    string(task.Status),
		CreatedAt: task.CreatedAt,
	})
}

// GetTaskResult 处理任务结果查询请求
func (h *productHandler) GetTaskResult(c *gin.Context) {
	taskID := c.Param("task_id")

	if taskID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "task_id is required",
		})
		return
	}

	result, err := h.productService.GetTaskResult(c.Request.Context(), taskID)
	if err != nil {
		logger.GetGlobalLogger("productenrich/handler.go").WithField("task_id", taskID).WithError(err).Error("failed to get task result")

		if errors.Is(err, ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "task_not_found",
				Message: "Task with the specified ID does not exist",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "query_failed",
			Message: "查询失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
