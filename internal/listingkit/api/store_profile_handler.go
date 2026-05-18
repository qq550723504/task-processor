package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) ListSheinStoreProfiles(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	items, err := h.service.ListSheinStoreProfiles(requestContext(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_profile_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) UpsertSheinStoreProfile(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	var req listingkit.ListingKitStoreProfile
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	profile, err := h.service.UpsertSheinStoreProfile(requestContext(c), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_profile_upsert_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *handler) DeleteSheinStoreProfile(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "invalid profile id"})
		return
	}
	if err := h.service.DeleteSheinStoreProfile(requestContext(c), id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingkit.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "store_profile_delete_failed", "message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handler) GetSheinStoreRoutingSettings(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	settings, err := h.service.GetSheinStoreRoutingSettings(requestContext(c))
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
	settings, err := h.service.UpdateSheinStoreRoutingSettings(requestContext(c), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_routing_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}
