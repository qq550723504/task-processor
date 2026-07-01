package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) ListAdminScheduledTaskConfigs(c *gin.Context) {
	if !h.requireScheduledTaskConfigHandler(c) {
		return
	}
	h.scheduledTaskConfigHandler.ListScheduledTaskConfigs(c)
}

func (h *handler) GetAdminScheduledTaskConfig(c *gin.Context) {
	if !h.requireScheduledTaskConfigHandler(c) {
		return
	}
	h.scheduledTaskConfigHandler.GetScheduledTaskConfig(c)
}

func (h *handler) UpsertAdminScheduledTaskConfig(c *gin.Context) {
	if !h.requireScheduledTaskConfigHandler(c) {
		return
	}
	h.scheduledTaskConfigHandler.UpsertScheduledTaskConfig(c)
}

func (h *handler) UpdateAdminScheduledTaskConfigStatus(c *gin.Context) {
	if !h.requireScheduledTaskConfigHandler(c) {
		return
	}
	h.scheduledTaskConfigHandler.UpdateScheduledTaskConfigStatus(c)
}

func (h *handler) DeleteAdminScheduledTaskConfig(c *gin.Context) {
	if !h.requireScheduledTaskConfigHandler(c) {
		return
	}
	h.scheduledTaskConfigHandler.DeleteScheduledTaskConfig(c)
}

func (h *handler) requireScheduledTaskConfigHandler(c *gin.Context) bool {
	if h.scheduledTaskConfigHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "scheduled_task_config_repository_unavailable",
		"message": "ListingKit scheduled task config repository is not configured",
	})
	return false
}
