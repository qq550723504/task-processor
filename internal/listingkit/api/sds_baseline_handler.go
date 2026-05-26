package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type sdsBaselineReadinessRequest struct {
	TenantID         string `form:"tenant_id"`
	ParentProductID  int64  `form:"parent_product_id"`
	PrototypeGroupID int64  `form:"prototype_group_id"`
	VariantID        int64  `form:"variant_id"`
}

type warmSDSBaselineRequest struct {
	TenantID  string                     `json:"tenant_id"`
	ImageURLs []string                   `json:"image_urls"`
	SDS       *listingkit.SDSSyncOptions `json:"sds"`
}

func (h *handler) GetSDSBaselineReadiness(c *gin.Context) {
	var req sdsBaselineReadinessRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	selectedVariantIDs, err := parseSDSBaselineSelectedVariantIDs(c.Query("selected_variant_ids"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	query := &listingkit.SDSBaselineReadinessQuery{
		TenantID:           requestTenantID(c, req.TenantID),
		ParentProductID:    req.ParentProductID,
		PrototypeGroupID:   req.PrototypeGroupID,
		VariantID:          req.VariantID,
		SelectedVariantIDs: selectedVariantIDs,
	}
	readiness, readinessErr := h.taskLifecycleService.GetSDSBaselineReadiness(
		requestContext(c, query.TenantID),
		query,
	)
	if readinessErr != nil {
		status := http.StatusInternalServerError
		if strings.Contains(readinessErr.Error(), "must be positive") || strings.Contains(readinessErr.Error(), "cannot be nil") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "sds_baseline_readiness_failed", "message": readinessErr.Error()})
		return
	}
	c.JSON(http.StatusOK, readiness)
}

func parseSDSBaselineSelectedVariantIDs(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	result := make([]int64, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		value, err := strconv.ParseInt(token, 10, 64)
		if err != nil || value <= 0 {
			return nil, strconv.ErrSyntax
		}
		result = append(result, value)
	}
	return result, nil
}

func (h *handler) WarmSDSBaseline(c *gin.Context) {
	if h.sdsBaselineWarmService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_baseline_warmup_unavailable"})
		return
	}
	var req warmSDSBaselineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c, req.TenantID)
	readiness, warmErr := h.sdsBaselineWarmService.WarmSDSBaseline(
		requestContext(c, req.TenantID),
		&listingkit.WarmSDSBaselineRequest{
			TenantID:  req.TenantID,
			ImageURLs: req.ImageURLs,
			SDS:       req.SDS,
		},
	)
	if warmErr != nil {
		status := http.StatusInternalServerError
		if strings.Contains(warmErr.Error(), "must be positive") || strings.Contains(warmErr.Error(), "cannot be nil") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "sds_baseline_warmup_failed", "message": warmErr.Error()})
		return
	}
	c.JSON(http.StatusOK, readiness)
}
