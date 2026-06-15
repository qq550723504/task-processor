package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (h *handler) TriggerSheinStoreSync(c *gin.Context) {
	if h.sheinSyncService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_sync_unavailable", "message": "SHEIN sync service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var req triggerSheinStoreSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if req.TriggerMode == "" {
		req.TriggerMode = listingkit.SheinSyncTriggerModeManual
	}

	job, err := h.sheinSyncService.SyncSheinOnShelfProducts(ctx, tenantID, storeID, req.TriggerMode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_sync_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *handler) ListSheinSyncedProducts(c *gin.Context) {
	if h.sheinSyncService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_sync_unavailable", "message": "SHEIN sync service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var query listSheinSyncedProductsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	activePtr, err := parseOptionalBoolQuery(query.IsActive)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	items, total, err := h.sheinSyncService.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
		TenantID: tenantID,
		StoreID:  storeID,
		SKCName:  strings.TrimSpace(query.SKCName),
		IsActive: activePtr,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_synced_products_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *handler) UpdateSheinSyncedProductCost(c *gin.Context) {
	if h.sheinSyncService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_sync_unavailable", "message": "SHEIN sync service is not configured"})
		return
	}

	productID, err := parseSheinInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	var req updateSheinSyncedProductCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	if err := h.sheinSyncService.UpdateManualCostPrice(requestContext(c), productID, req.ManualCostPrice); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "shein_synced_product_cost_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": productID, "manual_cost_price": req.ManualCostPrice})
}
