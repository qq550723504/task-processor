package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) ClearSheinResolutionCache(c *gin.Context) {
	result, err := h.service.ClearSheinResolutionCache(requestContext(c), c.Param("task_id"), c.DefaultQuery("kind", "all"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrInvalidSheinResolutionCacheKind):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "shein_resolution_cache_clear_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
