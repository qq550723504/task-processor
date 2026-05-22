package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GenerateStudioDesigns(c *gin.Context) {
	if !h.requireSubscriptionUsage(c, listingsubscription.ModuleStudio, "design_jobs", 1) {
		return
	}
	var req listingkit.StudioDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	response, err := h.studioMediaService.GenerateStudioDesigns(requestContext(c), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "studio_design_generation_failed", "message": err.Error()})
		return
	}
	for idx := range response.Images {
		response.Images[idx].ImageURL = absolutizeUploadedImageURLs(c, []string{response.Images[idx].ImageURL})[0]
	}

	c.JSON(http.StatusOK, response)
}
