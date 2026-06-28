package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

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
		ExecutableOnly:   query.ExecutableOnly,
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
