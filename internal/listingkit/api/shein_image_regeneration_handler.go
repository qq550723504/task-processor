package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) RegenerateSheinDataImage(c *gin.Context) {
	var req listingkit.RegenerateSheinDataImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	response, err := h.service.RegenerateSheinDataImage(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable):
			status = http.StatusNotFound
		case strings.Contains(err.Error(), "invalid request"):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "shein_image_regeneration_failed", "message": err.Error()})
		return
	}
	if strings.TrimSpace(response.Image.ImageURL) != "" {
		response.Image.ImageURL = absolutizeUploadedImageURLs(c, []string{response.Image.ImageURL})[0]
	}

	c.JSON(http.StatusOK, response)
}
