package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

func (h *handler) SearchSheinCategories(c *gin.Context) {
	result, err := h.storeAdminService.SearchSheinCategories(
		requestContext(c),
		c.Param("task_id"),
		c.Query("query"),
	)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, core.ErrTaskNotFound):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrInvalidSheinCategorySearchQuery):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "shein_category_search_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
