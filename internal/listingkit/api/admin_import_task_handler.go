package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminImportTasks(c *gin.Context) {
	if !h.requireImportTaskHandler(c) {
		return
	}
	h.importTaskHandler.ListImportTasks(c)
}

func (h *handler) BatchCreateAdminImportTasks(c *gin.Context) {
	if !h.requireImportTaskHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	h.importTaskHandler.BatchCreateImportTasks(c)
}

func (h *handler) DeleteAdminImportTask(c *gin.Context) {
	if !h.requireImportTaskHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	h.importTaskHandler.DeleteImportTask(c)
}

func (h *handler) requireImportTaskHandler(c *gin.Context) bool {
	if h.importTaskHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "import_task_repository_unavailable",
		"message": "ListingKit import task repository is not configured",
	})
	return false
}
