package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminGenerationTopicCatalog(c *gin.Context) {
	if !h.requireGenerationTopicCatalogHandler(c) {
		return
	}
	h.generationTopicCatalogHandler.ListGenerationTopicCatalog(c)
}

func (h *handler) ListAdminGenerationTopicOverrides(c *gin.Context) {
	if !h.requireGenerationTopicOverrideHandler(c) {
		return
	}
	h.generationTopicOverrideHandler.ListGenerationTopicOverrides(c)
}

func (h *handler) GetAdminGenerationTopicOverride(c *gin.Context) {
	if !h.requireGenerationTopicOverrideHandler(c) {
		return
	}
	h.generationTopicOverrideHandler.GetGenerationTopicOverride(c)
}

func (h *handler) CreateAdminGenerationTopicOverride(c *gin.Context) {
	if !h.requireGenerationTopicOverrideHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicOverrideHandler.CreateGenerationTopicOverride(c)
}

func (h *handler) UpdateAdminGenerationTopicOverride(c *gin.Context) {
	if !h.requireGenerationTopicOverrideHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicOverrideHandler.UpdateGenerationTopicOverride(c)
}

func (h *handler) UpdateAdminGenerationTopicOverrideStatus(c *gin.Context) {
	if !h.requireGenerationTopicOverrideHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicOverrideHandler.UpdateGenerationTopicOverrideStatus(c)
}

func (h *handler) DeleteAdminGenerationTopicOverride(c *gin.Context) {
	if !h.requireGenerationTopicOverrideHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicOverrideHandler.DeleteGenerationTopicOverride(c)
}

func (h *handler) requireGenerationTopicCatalogHandler(c *gin.Context) bool {
	if h.generationTopicCatalogHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "generation_topic_catalog_unavailable",
		"message": "ListingKit generation topic catalog handler is not configured",
	})
	return false
}

func (h *handler) requireGenerationTopicOverrideHandler(c *gin.Context) bool {
	if h.generationTopicOverrideHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "generation_topic_override_repository_unavailable",
		"message": "ListingKit generation topic override repository is not configured",
	})
	return false
}
