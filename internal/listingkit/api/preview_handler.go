package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GetTaskPreview(c *gin.Context) {
	preview, err := h.service.GetTaskPreview(c.Request.Context(), c.Param("task_id"), c.Query("platform"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrUnsupportedPreviewPlatform):
			status = http.StatusBadRequest
		case errors.Is(err, listingkit.ErrPreviewPlatformUnavailable):
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "preview_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}
