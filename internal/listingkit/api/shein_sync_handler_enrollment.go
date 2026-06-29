package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (h *handler) ExecuteSheinActivityEnrollment(c *gin.Context) {
	if h.sheinEnrollmentService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_enrollment_unavailable", "message": "SHEIN enrollment service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var req executeSheinActivityEnrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if strings.TrimSpace(req.ActivityType) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "activity_type is required"})
		return
	}
	if req.TriggerMode == "" {
		req.TriggerMode = listingkit.SheinEnrollmentRunTriggerModeManualConfirmed
	}

	run, err := h.sheinEnrollmentService.StartSheinActivityEnrollment(
		ctx,
		tenantID,
		storeID,
		strings.TrimSpace(req.ActivityType),
		strings.TrimSpace(req.ActivityKey),
		req.TriggerMode,
		req.CandidateIDs...,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "shein_enrollment_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run": run})
}
