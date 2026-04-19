package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"task-processor/internal/listingkit"
)

func (h *handler) ValidateTaskRevision(c *gin.Context) {
	var req listingkit.ApplyRevisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	result, err := h.service.ValidateTaskRevision(c.Request.Context(), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == listingkit.ErrTaskNotFound || err == listingkit.ErrTaskResultUnavailable || err == listingkit.ErrRevisionHistoryRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "revision_validate_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
