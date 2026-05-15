package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminProductImportMappings(c *gin.Context) {
	if !h.requireProductImportMappingHandler(c) {
		return
	}
	h.productImportMappingHandler.ListProductImportMappings(c)
}

func (h *handler) GetAdminProductImportMapping(c *gin.Context) {
	if !h.requireProductImportMappingHandler(c) {
		return
	}
	h.productImportMappingHandler.GetProductImportMapping(c)
}

func (h *handler) CreateAdminProductImportMapping(c *gin.Context) {
	if !h.requireProductImportMappingHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	h.productImportMappingHandler.CreateProductImportMapping(c)
}

func (h *handler) UpdateAdminProductImportMapping(c *gin.Context) {
	if !h.requireProductImportMappingHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	h.productImportMappingHandler.UpdateProductImportMapping(c)
}

func (h *handler) UpdateAdminProductImportMappingStatus(c *gin.Context) {
	if !h.requireProductImportMappingHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	h.productImportMappingHandler.UpdateProductImportMappingStatus(c)
}

func (h *handler) DeleteAdminProductImportMapping(c *gin.Context) {
	if !h.requireProductImportMappingHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	h.productImportMappingHandler.DeleteProductImportMapping(c)
}

func (h *handler) requireProductImportMappingHandler(c *gin.Context) bool {
	if h.productImportMappingHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "product_import_mapping_repository_unavailable",
		"message": "ListingKit product import mapping repository is not configured",
	})
	return false
}
