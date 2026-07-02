package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) AnalyzeStudioReferenceStyle(c *gin.Context) {
	var req listingkit.StudioReferenceAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if h.studioMediaService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "reference_analysis_unavailable", "message": "studio media service is not configured"})
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
