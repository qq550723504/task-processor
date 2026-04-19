package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) ApplyTaskRevision(c *gin.Context) {
	var req listingkit.ApplyRevisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	preview, err := h.service.ApplyTaskRevision(c.Request.Context(), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		var validationErr *listingkit.RevisionValidationError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrPreviewPlatformUnavailable), errors.Is(err, listingkit.ErrTaskResultUnavailable), errors.Is(err, listingkit.ErrRevisionHistoryRecordNotFound):
			status = http.StatusNotFound
		case errors.As(err, &validationErr):
			status = http.StatusBadRequest
		case errors.Is(err, listingkit.ErrUnsupportedPreviewPlatform), errors.Is(err, listingkit.ErrInvalidRevisionRequest):
			status = http.StatusBadRequest
		}
		payload := gin.H{"error": "revision_apply_failed", "message": err.Error()}
		if validationErr != nil {
			payload["field_errors"] = validationErr.Fields
		}
		c.JSON(status, payload)
		return
	}

	c.JSON(http.StatusOK, preview)
}
