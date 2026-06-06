package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) RecoverTaskNow(c *gin.Context) {
	if h.taskRecoveryService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "task_recovery_not_supported", "message": "task recovery is not supported"})
		return
	}
	task, err := h.taskRecoveryService.RecoverTaskNow(requestContext(c), c.Param("task_id"))
	if err != nil {
		c.JSON(taskRecoveryErrorStatus(err), gin.H{"error": "task_recovery_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *handler) BulkRecoverTasks(c *gin.Context) {
	if h.taskRecoveryService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "task_recovery_not_supported", "message": "task recovery is not supported"})
		return
	}
	var req listingkit.RecoverBlockedTasksQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	recoveredCount, err := h.taskRecoveryService.BulkRecoverTasks(requestContext(c), &req)
	if err != nil {
		c.JSON(taskRecoveryErrorStatus(err), gin.H{"error": "task_recovery_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovered_count": recoveredCount})
}

func taskRecoveryErrorStatus(err error) int {
	switch {
	case errors.Is(err, listingkit.ErrTaskNotFound):
		return http.StatusNotFound
	case errors.Is(err, listingkit.ErrTaskNotRecoverable):
		return http.StatusConflict
	case errors.Is(err, listingkit.ErrTaskRecoveryUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
