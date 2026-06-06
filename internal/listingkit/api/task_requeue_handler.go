package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) RequeuePendingTasks(c *gin.Context) {
	if h.taskRequeueService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "task_requeue_not_supported", "message": "task requeue is not supported"})
		return
	}
	var req listingkit.RequeuePendingTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if len(req.TaskIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": listingkit.ErrTaskRequeueInvalidRequest.Error()})
		return
	}
	result, err := h.taskRequeueService.RequeuePendingTasks(requestContext(c), &req)
	if err != nil {
		c.JSON(taskRequeueErrorStatus(err), gin.H{"error": "task_requeue_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func taskRequeueErrorStatus(err error) int {
	switch {
	case errors.Is(err, listingkit.ErrTaskRequeueInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, listingkit.ErrTaskRequeueUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
