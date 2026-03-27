package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	productimage "task-processor/internal/productimage"
)

type imageHandler struct {
	service productimage.HandlerService
}

func NewImageHandler(service productimage.HandlerService) (productimage.Handler, error) {
	if service == nil {
		return nil, fmt.Errorf("image service cannot be nil")
	}
	return &imageHandler{service: service}, nil
}

func (h *imageHandler) ProcessImages(c *gin.Context) {
	var req productimage.ImageProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("invalid image process request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	task, err := h.service.CreateProcessTask(c.Request.Context(), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "task_creation_failed"
		message := "image processing task creation failed"
		if isInvalidProcessRequest(err) {
			statusCode = http.StatusBadRequest
			errorCode = "invalid_request"
			message = err.Error()
		}
		c.JSON(statusCode, gin.H{"error": errorCode, "message": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":    task.ID,
		"status":     task.Status,
		"created_at": task.CreatedAt,
	})
}

func (h *imageHandler) GetTaskResult(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "task_id is required"})
		return
	}

	result, err := h.service.GetTaskResult(c.Request.Context(), taskID)
	if err != nil {
		if errors.Is(err, productimage.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task_not_found", "message": "Task with the specified ID does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed", "message": "image task query failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *imageHandler) ReviewTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "task_id is required"})
		return
	}

	var req productimage.ReviewTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Warn("invalid image review request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	result, err := h.service.ReviewTask(c.Request.Context(), taskID, &req)
	if err != nil {
		if errors.Is(err, productimage.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task_not_found", "message": "Task with the specified ID does not exist"})
			return
		}
		if isInvalidProcessRequest(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "review_failed", "message": "image task review failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func isInvalidProcessRequest(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid request:") || strings.Contains(msg, "request cannot be nil")
}
