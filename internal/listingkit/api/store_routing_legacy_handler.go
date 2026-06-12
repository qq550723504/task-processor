package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GetSheinStoreRoutingSettings(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	settings, err := h.storeAdminService.GetSheinStoreRoutingSettings(requestContext(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_routing_read_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *handler) UpdateSheinStoreRoutingSettings(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	var req listingkit.ListingKitStoreRoutingSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	settings, err := h.storeAdminService.UpdateSheinStoreRoutingSettings(requestContext(c), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_routing_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}
