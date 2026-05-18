package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListTenantStores(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.ListStores(c)
}

func (h *handler) CreateTenantStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.CreateStore(c)
}

func (h *handler) UpdateTenantStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.UpdateStore(c)
}

func (h *handler) DeleteTenantStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.DeleteStore(c)
}

func (h *handler) ListSimpleTenantStores(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.ListSimpleStores(c)
}

func (h *handler) requireTenantStoreHandler(c *gin.Context) bool {
	if h.storeHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "store_repository_unavailable",
		"message": "ListingKit store repository is not configured",
	})
	return false
}
