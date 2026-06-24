package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) CreateSDSRetirementRun(c *gin.Context) {
	if h.sdsRetirementService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_retirement_unavailable"})
		return
	}

	var req listingkit.CreateSDSRetirementRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c, req.TenantID)

	detail, err := h.sdsRetirementService.CreateSDSRetirementRun(requestContext(c, req.TenantID), &req)
	respondSDSRetirement(c, detail, err)
}

func (h *handler) GetSDSRetirementRun(c *gin.Context) {
	if h.sdsRetirementService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_retirement_unavailable"})
		return
	}

	ctx, ok := requireExplicitRequestContext(c)
	if !ok {
		return
	}

	detail, err := h.sdsRetirementService.GetSDSRetirementRun(ctx, c.Param("run_id"))
	respondSDSRetirement(c, detail, err)
}

func (h *handler) UpdateSDSRetirementSelection(c *gin.Context) {
	if h.sdsRetirementService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_retirement_unavailable"})
		return
	}

	var req listingkit.UpdateSDSRetirementSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	ctx, ok := requireExplicitRequestContext(c)
	if !ok {
		return
	}

	detail, err := h.sdsRetirementService.UpdateSDSRetirementSelection(ctx, c.Param("run_id"), &req)
	respondSDSRetirement(c, detail, err)
}

func (h *handler) ConfirmSDSRetirementRun(c *gin.Context) {
	if h.sdsRetirementService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_retirement_unavailable"})
		return
	}

	var req listingkit.ConfirmSDSRetirementRunRequest
	_ = c.ShouldBindJSON(&req)

	ctx, ok := requireExplicitRequestContext(c)
	if !ok {
		return
	}

	detail, err := h.sdsRetirementService.ConfirmSDSRetirementRun(ctx, c.Param("run_id"), &req)
	respondSDSRetirement(c, detail, err)
}

func (h *handler) RetrySDSRetirementRun(c *gin.Context) {
	if h.sdsRetirementService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sds_retirement_unavailable"})
		return
	}

	ctx, ok := requireExplicitRequestContext(c)
	if !ok {
		return
	}

	detail, err := h.sdsRetirementService.RetrySDSRetirementRun(ctx, c.Param("run_id"))
	respondSDSRetirement(c, detail, err)
}

func respondSDSRetirement(c *gin.Context, detail *listingkit.SDSRetirementRunDetail, err error) {
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sds_retirement_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}
