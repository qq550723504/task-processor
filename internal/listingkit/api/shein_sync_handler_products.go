package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type sheinSDSCostGroupHandlerService interface {
	ListSDSCostGroups(ctx context.Context, query *listingkit.SheinSDSCostGroupQuery) ([]listingkit.SheinSDSCostGroupRecord, int64, error)
	UpdateSDSCostGroupManualCost(ctx context.Context, tenantID, storeID int64, groupKey, groupLabel string, manualCostPrice *float64) (*listingkit.SheinSDSCostGroupRecord, error)
}

type sheinSourceSDSMetadataHandlerService interface {
	ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error)
}

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

func (h *handler) ListSheinSDSCostGroups(c *gin.Context) {
	service, ok := h.sheinSyncService.(sheinSDSCostGroupHandlerService)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_sds_cost_groups_unavailable", "message": "SHEIN SDS cost group service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var query listSheinSDSCostGroupsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	items, total, err := service.ListSDSCostGroups(ctx, &listingkit.SheinSDSCostGroupQuery{
		TenantID: tenantID,
		StoreID:  storeID,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_sds_cost_groups_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *handler) ListSheinSourceSDSMetadata(c *gin.Context) {
	service, ok := h.taskLifecycleService.(sheinSourceSDSMetadataHandlerService)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_source_sds_metadata_unavailable", "message": "SHEIN source SDS metadata service is not configured"})
		return
	}

	storeID, _, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	sourceCodes := parseSheinSourceSDSMetadataCodes(c)
	if len(sourceCodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "source_codes is required"})
		return
	}
	items, err := service.ListSheinSourceSDSMetadata(ctx, &listingkit.SheinSourceSDSMetadataQuery{
		StoreID:     storeID,
		SourceCodes: sourceCodes,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_source_sds_metadata_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) UpdateSheinSDSCostGroup(c *gin.Context) {
	service, ok := h.sheinSyncService.(sheinSDSCostGroupHandlerService)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_sds_cost_groups_unavailable", "message": "SHEIN SDS cost group service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}
	groupKey, err := url.PathUnescape(strings.TrimSpace(c.Param("group_key")))
	if err != nil || strings.TrimSpace(groupKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "group_key is required"})
		return
	}

	var req updateSheinSDSCostGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	group, err := service.UpdateSDSCostGroupManualCost(ctx, tenantID, storeID, groupKey, req.GroupLabel, req.ManualCostPrice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_sds_cost_group_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"group": group})
}

func parseSheinSourceSDSMetadataCodes(c *gin.Context) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, value := range append(c.QueryArray("source_codes"), c.QueryArray("source_code")...) {
		for _, part := range strings.Split(value, ",") {
			code := strings.TrimSpace(part)
			if code == "" {
				continue
			}
			key := strings.ToUpper(code)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, code)
			if len(out) >= 100 {
				return out
			}
		}
	}
	return out
}
