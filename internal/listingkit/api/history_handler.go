package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GetTaskRevisionHistory(c *gin.Context) {
	var query listingkit.RevisionHistoryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	page, err := h.service.GetTaskRevisionHistory(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrInvalidRevisionHistoryCursor), errors.Is(err, listingkit.ErrInvalidRevisionHistoryActionType):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "revision_history_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}
