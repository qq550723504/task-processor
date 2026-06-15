package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) ListSheinActivityEnrollmentRuns(c *gin.Context) {
	if h.sheinSyncRepository == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_enrollment_runs_unavailable", "message": "SHEIN enrollment run repository is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var query listSheinEnrollmentRunsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	items, total, err := h.sheinSyncRepository.ListEnrollmentRuns(ctx, &listingkit.SheinEnrollmentRunQuery{
		TenantID:     tenantID,
		StoreID:      storeID,
		ActivityType: strings.TrimSpace(query.ActivityType),
		ActivityKey:  strings.TrimSpace(query.ActivityKey),
		Page:         query.Page,
		PageSize:     query.PageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_enrollment_runs_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}
