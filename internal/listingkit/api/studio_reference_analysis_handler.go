package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) AnalyzeStudioReferenceStyle(c *gin.Context) {
	if !h.requireSubscriptionUsage(c, listingsubscription.ModuleStudio, "design_jobs", 1) {
		return
	}
	var req listingkit.StudioReferenceAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	response, err := h.studioMediaService.AnalyzeStudioReferenceStyle(requestContext(c), &req)
	if err != nil {
		status := http.StatusInternalServerError
		errorCode := "reference_analysis_failed"
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
			errorCode = "invalid_request"
		}
		if strings.Contains(err.Error(), "reference_analysis_unavailable") {
			status = http.StatusNotImplemented
			errorCode = "reference_analysis_unavailable"
		}
		c.JSON(status, gin.H{"error": errorCode, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
