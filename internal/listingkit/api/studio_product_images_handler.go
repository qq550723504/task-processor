package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GenerateStudioProductImages(c *gin.Context) {
	var req listingkit.StudioProductImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	response, err := h.service.GenerateStudioProductImages(requestContext(c), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "studio_product_images_failed", "message": err.Error()})
		return
	}
	for idx := range response.Images {
		response.Images[idx].ImageURL = absolutizeUploadedImageURLs(c, []string{response.Images[idx].ImageURL})[0]
	}

	c.JSON(http.StatusOK, response)
}
