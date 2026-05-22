package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GetTaskExport(c *gin.Context) {
	export, err := h.taskLifecycleService.GetTaskExport(requestContext(c), c.Param("task_id"), c.Query("platform"))
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
		c.JSON(status, gin.H{"error": "export_query_failed", "message": err.Error()})
		return
	}

	c.Header("Content-Type", export.MimeType)
	c.Header("Content-Disposition", `attachment; filename="`+export.FileName+`"`)
	c.JSON(http.StatusOK, export)
}
