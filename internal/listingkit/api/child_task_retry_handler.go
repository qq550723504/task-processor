package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

func (h *handler) RetryTaskChildTask(c *gin.Context) {
	if h.childTaskRetryService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "child_task_retry_not_supported", "message": "child task retry is not supported"})
		return
	}
	var req listingkit.RetryChildTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if strings.TrimSpace(req.Kind) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "kind is required"})
		return
	}
	result, err := h.childTaskRetryService.ScheduleTaskChildRetry(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, core.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable), errors.Is(err, core.ErrChildTaskNotFound):
			status = http.StatusNotFound
		case errors.Is(err, core.ErrChildTaskRetryInvalidRequest):
			status = http.StatusBadRequest
		case errors.Is(err, core.ErrChildTaskNotRetryable), errors.Is(err, core.ErrChildTaskRetryConflict), errors.Is(err, listingkit.ErrSDSRepairRetryInProgress):
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "child_task_retry_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, result)
}
