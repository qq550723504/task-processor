package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminStores(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	h.storeHandler.ListStores(c)
}

func (h *handler) GetAdminStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	h.storeHandler.GetStore(c)
}

func (h *handler) CreateAdminStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.CreateStore(c)
}

func (h *handler) UpdateAdminStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.UpdateStore(c)
}

func (h *handler) UpdateAdminStoreStatus(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.UpdateStoreStatus(c)
}

func (h *handler) DeleteAdminStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.DeleteStore(c)
}

func (h *handler) ListSimpleAdminStores(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	h.storeHandler.ListSimpleStores(c)
}

func (h *handler) ListAdminStoreStatistics(c *gin.Context) {
	if !h.requireStoreStatisticsHandler(c) {
		return
	}
	h.storeStatisticsHandler.ListStoreStatistics(c)
}

func (h *handler) GetAdminDispatchEventSummary(c *gin.Context) {
	if !h.requireDispatchEventHandler(c) {
		return
	}
	h.dispatchEventHandler.GetDispatchEventSummary(c)
}

func (h *handler) ListAdminDispatchEvents(c *gin.Context) {
	if !h.requireDispatchEventHandler(c) {
		return
	}
	h.dispatchEventHandler.ListDispatchEvents(c)
}

func (h *handler) ListDeletedAdminStores(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	h.storeHandler.ListDeletedStores(c)
}

func (h *handler) RestoreAdminStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.RestoreStore(c)
}

func (h *handler) PermanentlyDeleteAdminStore(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.PermanentlyDeleteStore(c)
}

func (h *handler) ExtendAdminStoreValidity(c *gin.Context) {
	if !h.requireStoreHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleStoreManagement) {
		return
	}
	h.storeHandler.ExtendStoreValidity(c)
}

func (h *handler) requireStoreHandler(c *gin.Context) bool {
	if h.storeHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "store_repository_unavailable",
		"message": "ListingKit store repository is not configured",
	})
	return false
}

func (h *handler) requireStoreStatisticsHandler(c *gin.Context) bool {
	if h.storeStatisticsHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "store_statistics_repository_unavailable",
		"message": "ListingKit store statistics repository is not configured",
	})
	return false
}

func (h *handler) requireDispatchEventHandler(c *gin.Context) bool {
	if h.dispatchEventHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "dispatch_event_repository_unavailable",
		"message": "ListingKit dispatch event repository is not configured",
	})
	return false
}
