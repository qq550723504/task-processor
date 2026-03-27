package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/logger"
	"task-processor/internal/productenrich"
)

type productHandler struct {
	productService productenrich.ProductHandlerService
}

func NewProductHandler(productService productenrich.ProductHandlerService) (productenrich.ProductHandler, error) {
	if productService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}

	return &productHandler{productService: productService}, nil
}

func (h *productHandler) GenerateProduct(c *gin.Context) {
	var req productenrich.GenerateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("invalid request body")
		c.JSON(http.StatusBadRequest, productenrich.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	task, err := h.productService.CreateGenerateTask(c.Request.Context(), &req)
	if err != nil {
		logrus.WithError(err).Error("failed to create task")
		statusCode := http.StatusInternalServerError
		errorCode := "task_creation_failed"
		message := "任务创建失败，请稍后重试"
		if isInvalidGenerateRequest(err) {
			statusCode = http.StatusBadRequest
			errorCode = "invalid_request"
			message = err.Error()
		}
		c.JSON(statusCode, productenrich.ErrorResponse{
			Error:   errorCode,
			Message: message,
		})
		return
	}

	c.JSON(http.StatusOK, productenrich.TaskResponse{
		TaskID:    task.ID,
		Status:    string(task.Status),
		CreatedAt: task.CreatedAt,
	})
}

func (h *productHandler) GetTaskResult(c *gin.Context) {
	taskID := c.Param("task_id")

	if taskID == "" {
		c.JSON(http.StatusBadRequest, productenrich.ErrorResponse{
			Error:   "invalid_request",
			Message: "task_id is required",
		})
		return
	}

	result, err := h.productService.GetTaskResult(c.Request.Context(), taskID)
	if err != nil {
		logger.GetGlobalLogger("productenrich/api/handler.go").WithField("task_id", taskID).WithError(err).Error("failed to get task result")

		if errors.Is(err, productenrich.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, productenrich.ErrorResponse{
				Error:   "task_not_found",
				Message: "Task with the specified ID does not exist",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, productenrich.ErrorResponse{
			Error:   "query_failed",
			Message: "查询失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func isInvalidGenerateRequest(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid request:") || strings.Contains(msg, "request cannot be nil")
}
