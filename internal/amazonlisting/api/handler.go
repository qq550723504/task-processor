package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
)

type handler struct {
	service amazonlisting.HandlerService
}

func NewHandler(service amazonlisting.HandlerService) (amazonlisting.Handler, error) {
	if service == nil {
		return nil, errors.New("service cannot be nil")
	}
	return &handler{service: service}, nil
}

func (h *handler) GenerateListing(c *gin.Context) {
	var req amazonlisting.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	task, err := h.service.CreateGenerateTask(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") || strings.Contains(err.Error(), "supported currently") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "task_creation_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "status": task.Status, "created_at": task.CreatedAt})
}

func (h *handler) GetTaskResult(c *gin.Context) {
	result, err := h.service.GetTaskResult(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, amazonlisting.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *handler) GetTaskWorkbench(c *gin.Context) {
	result, err := h.service.GetTaskWorkbench(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, amazonlisting.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *handler) ReviewTask(c *gin.Context) {
	var req amazonlisting.ReviewTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.service.ReviewTask(c.Request.Context(), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, amazonlisting.ErrTaskNotFound) {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "unsupported review action") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "review_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *handler) SubmitTask(c *gin.Context) {
	var req amazonlisting.SubmitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.service.SubmitTask(c.Request.Context(), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, amazonlisting.ErrTaskNotFound) {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "unsupported submit action") ||
			strings.Contains(err.Error(), "not configured") ||
			strings.Contains(err.Error(), "empty") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "submit_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
