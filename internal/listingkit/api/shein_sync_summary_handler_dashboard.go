package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"task-processor/internal/listingkit"
)

func (h *handler) ListSheinEnrollmentDashboard(c *gin.Context) {
	if h.storeRepository == nil || h.sheinSyncRepository == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_summary_unavailable", "message": "SHEIN summary dependencies are not configured"})
		return
	}

	tenantID, err := parseSheinTenantID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	var query sheinSummaryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	activityType := resolveSheinSummaryActivityType(query.ActivityType)
	ctx := requestContext(c, strconv.FormatInt(tenantID, 10))

	stores, err := h.listSheinStores(ctx, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_dashboard_list_failed", "message": err.Error()})
		return
	}

	items := make([]*listingkit.SheinEnrollmentStoreSummary, len(stores))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(sheinSummaryConcurrency)
	for i := range stores {
		index := i
		store := stores[i]
		group.Go(func() error {
			summary, summaryErr := h.buildSheinEnrollmentStoreSummary(groupCtx, tenantID, &store, activityType)
			if summaryErr != nil {
				return summaryErr
			}
			items[index] = summary
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_dashboard_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":         items,
		"total":         len(items),
		"activity_type": activityType,
	})
}

func (h *handler) GetSheinEnrollmentStoreSummary(c *gin.Context) {
	if h.storeRepository == nil || h.sheinSyncRepository == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_summary_unavailable", "message": "SHEIN summary dependencies are not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var query sheinSummaryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	activityType := resolveSheinSummaryActivityType(query.ActivityType)

	store, err := h.storeRepository.GetStore(ctx, tenantID, storeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_store_summary_failed", "message": err.Error()})
		return
	}

	summary, err := h.buildSheinEnrollmentStoreSummary(ctx, tenantID, store, activityType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_store_summary_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}
