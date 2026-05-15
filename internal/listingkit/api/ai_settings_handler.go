package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GetAIClientSettings(c *gin.Context) {
	settings, err := h.service.GetAIClientSettings(requestContext(c), c.Query("scope"), c.Query("client_name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ai_settings_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *handler) UpdateAIClientSettings(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	var req listingkit.AIClientSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	settings, err := h.service.UpdateAIClientSettings(requestContext(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ai_settings_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}
