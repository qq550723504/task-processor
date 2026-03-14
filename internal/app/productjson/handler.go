// Package productjson 提供 HTTP API 处理器
package productjson

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	domain "task-processor/internal/domain/productjson"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ProductHandlerService HTTP handler 依赖的产品服务接口
type ProductHandlerService interface {
	CreateGenerateTask(ctx context.Context, req *domain.GenerateRequest) (*domain.Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*domain.TaskResult, error)
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
	var req domain.GenerateRequest

	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("invalid request body")

		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// 创建任务
	task, err := h.productService.CreateGenerateTask(c.Request.Context(), &req)
	if err != nil {
		logrus.WithError(err).Error("failed to create task")

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "task_creation_failed",
			Message: err.Error(),
		})
		return
	}

	// 返回成功响应
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

	// 查询任务结果
	result, err := h.productService.GetTaskResult(c.Request.Context(), taskID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"task_id": taskID,
		}).WithError(err).Error("failed to get task result")

		// 判断是否是任务不存在
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "task_not_found",
				Message: "Task with the specified ID does not exist",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "query_failed",
			Message: err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, result)
}

// isNotFoundError 判断是否是未找到错误
func isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "task not found" ||
		strings.Contains(err.Error(), "not found"))
}
