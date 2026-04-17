package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GetTaskRevisionHistoryDetail(c *gin.Context) {
	var query listingkit.RevisionHistoryDetailQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	detail, err := h.service.GetTaskRevisionHistoryDetail(c.Request.Context(), c.Param("task_id"), c.Param("revision_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable), errors.Is(err, listingkit.ErrRevisionHistoryRecordNotFound), errors.Is(err, listingkit.ErrRevisionHistoryCompareTargetNotFound):
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "revision_history_detail_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}
