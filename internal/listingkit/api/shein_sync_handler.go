package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
	"task-processor/internal/tenantbridge"
)

type triggerSheinStoreSyncRequest struct {
	TriggerMode listingkit.SheinSyncTriggerMode `json:"trigger_mode"`
}

type listSheinSyncedProductsQuery struct {
	SKCName  string `form:"skc_name"`
	IsActive string `form:"is_active"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type updateSheinSyncedProductCostRequest struct {
	ManualCostPrice *float64 `json:"manual_cost_price"`
}

type refreshSheinActivityCandidatesRequest struct {
	ActivityType string `json:"activity_type"`
}

type listSheinActivityCandidatesQuery struct {
	ActivityType     string `form:"activity_type"`
	ActivityKey      string `form:"activity_key"`
	SKCName          string `form:"skc_name"`
	CandidateVersion string `form:"candidate_version"`
	Page             int    `form:"page"`
	PageSize         int    `form:"page_size"`
}

type reviewSheinActivityCandidateRequest struct {
	StoreID          int64                                 `json:"store_id"`
	ReviewStatus     listingkit.SheinCandidateReviewStatus `json:"review_status"`
	AutoModeEligible *bool                                 `json:"auto_mode_eligible"`
	SelectedForRun   *bool                                 `json:"selected_for_run"`
}

type executeSheinActivityEnrollmentRequest struct {
	ActivityType string                                   `json:"activity_type"`
	ActivityKey  string                                   `json:"activity_key"`
	TriggerMode  listingkit.SheinEnrollmentRunTriggerMode `json:"trigger_mode"`
	CandidateIDs []int64                                  `json:"candidate_ids"`
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

func (h *handler) RefreshSheinActivityCandidates(c *gin.Context) {
	if h.sheinCandidateService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_candidate_unavailable", "message": "SHEIN candidate service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var req refreshSheinActivityCandidatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if strings.TrimSpace(req.ActivityType) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "activity_type is required"})
		return
	}

	result, err := h.sheinCandidateService.RefreshCandidates(ctx, tenantID, storeID, strings.TrimSpace(req.ActivityType))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_candidate_refresh_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h *handler) ListSheinActivityCandidates(c *gin.Context) {
	if h.sheinCandidateService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_candidate_unavailable", "message": "SHEIN candidate service is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}

	var query listSheinActivityCandidatesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if strings.TrimSpace(query.ActivityType) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "activity_type is required"})
		return
	}

	items, total, err := h.sheinCandidateService.ListCandidates(ctx, &listingkit.SheinActivityCandidateQuery{
		TenantID:         tenantID,
		StoreID:          storeID,
		ActivityType:     strings.TrimSpace(query.ActivityType),
		ActivityKey:      strings.TrimSpace(query.ActivityKey),
		SKCName:          strings.TrimSpace(query.SKCName),
		CandidateVersion: strings.TrimSpace(query.CandidateVersion),
		Page:             query.Page,
		PageSize:         query.PageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_candidates_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *handler) ReviewSheinActivityCandidate(c *gin.Context) {
	if h.sheinCandidateService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_candidate_unavailable", "message": "SHEIN candidate service is not configured"})
		return
	}

	candidateID, err := parseSheinInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	tenantID, err := parseSheinTenantID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	var req reviewSheinActivityCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if req.StoreID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "store_id is required"})
		return
	}
	if req.ReviewStatus == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "review_status is required"})
		return
	}

	row, err := h.sheinCandidateService.ReviewCandidate(
		requestContext(c, strconv.FormatInt(tenantID, 10)),
		tenantID,
		req.StoreID,
		candidateID,
		req.ReviewStatus,
		req.AutoModeEligible,
		req.SelectedForRun,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "shein_candidate_review_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"candidate": row})
}

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

	run, err := h.sheinEnrollmentService.ExecuteSheinActivityEnrollment(
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

func parseSheinScopedRequest(c *gin.Context) (storeID int64, tenantID int64, ctx context.Context, ok bool) {
	storeID, err := parseSheinInt64Param(c, "store_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return 0, 0, nil, false
	}
	tenantID, err = parseSheinTenantID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return 0, 0, nil, false
	}
	return storeID, tenantID, requestContext(c, strconv.FormatInt(tenantID, 10)), true
}

func parseSheinTenantID(c *gin.Context) (int64, error) {
	value := strings.TrimSpace(requestTenantID(c))
	if value == "" || value == listingkit.DefaultTenantID {
		return 0, errors.New("numeric tenant_id is required")
	}
	tenantID, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), value)
	if err != nil || tenantID <= 0 {
		return 0, errors.New("numeric tenant_id is required")
	}
	return tenantID, nil
}

func parseSheinInt64Param(c *gin.Context, name string) (int64, error) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		return 0, errors.New(name + " is required")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + name)
	}
	return parsed, nil
}

func parseOptionalBoolQuery(value string) (*bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return nil, errors.New("invalid is_active")
	}
	return &parsed, nil
}
