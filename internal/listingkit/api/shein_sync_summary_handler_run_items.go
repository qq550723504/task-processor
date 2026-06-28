package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) ListSheinActivityEnrollmentRunItems(c *gin.Context) {
	if h.sheinSyncRepository == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_enrollment_run_items_unavailable", "message": "SHEIN enrollment item repository is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	runID, err := strconv.ParseInt(strings.TrimSpace(c.Param("run_id")), 10, 64)
	if err != nil || runID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "run_id must be a positive integer"})
		return
	}

	var query listSheinEnrollmentItemsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	var status *listingkit.SheinEnrollmentItemStatus
	if trimmed := strings.TrimSpace(query.Status); trimmed != "" {
		parsed := listingkit.SheinEnrollmentItemStatus(trimmed)
		status = &parsed
	}

	items, total, err := h.sheinSyncRepository.ListEnrollmentItems(ctx, &listingkit.SheinEnrollmentItemQuery{
		TenantID:       tenantID,
		StoreID:        storeID,
		RunID:          runID,
		Status:         status,
		IncludePayload: query.IncludePayload,
		Page:           query.Page,
		PageSize:       query.PageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_enrollment_run_items_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}
