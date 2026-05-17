package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) GetAIClientSettings(c *gin.Context) {
	settings, err := h.settingsService.Get(requestContext(c), "ai", settingsNamespaceQuery{
		Scope:      c.Query("scope"),
		ClientName: c.Query("client_name"),
	})
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
	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if !json.Valid(payload) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	settings, err := h.settingsService.Update(requestContext(c), "ai", settingsNamespaceQuery{
		Scope:      c.Query("scope"),
		ClientName: c.Query("client_name"),
	}, payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ai_settings_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}
