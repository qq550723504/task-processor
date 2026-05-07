package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) SubmitTask(c *gin.Context) {
	var req listingkit.SubmitTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}

	preview, err := h.service.SubmitTask(c.Request.Context(), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrUnsupportedSubmitPlatform), errors.Is(err, listingkit.ErrSubmitBlocked):
			status = http.StatusBadRequest
		case errors.Is(err, listingkit.ErrSubmitInProgress):
			status = http.StatusConflict
		}
		body := gin.H{"error": "submit_failed", "message": err.Error()}
		var inProgress *listingkit.SubmitInProgressError
		if errors.As(err, &inProgress) {
			body["current_phase"] = inProgress.Phase
			body["current_request_id"] = inProgress.RequestID
			if inProgress.LeaseExpiresAt != nil {
				body["lease_expires_at"] = inProgress.LeaseExpiresAt
			}
		}
		c.JSON(status, body)
		return
	}

	c.JSON(http.StatusOK, preview)
}
