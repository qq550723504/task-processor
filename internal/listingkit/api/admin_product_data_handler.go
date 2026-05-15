package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) ListAdminProductData(c *gin.Context) {
	if !h.requireProductDataHandler(c) {
		return
	}
	h.productDataHandler.ListProductData(c)
}

func (h *handler) GetAdminProductData(c *gin.Context) {
	if !h.requireProductDataHandler(c) {
		return
	}
	h.productDataHandler.GetProductData(c)
}

func (h *handler) CreateAdminProductData(c *gin.Context) {
	if !h.requireProductDataHandler(c) {
		return
	}
	h.productDataHandler.CreateProductData(c)
}

func (h *handler) UpdateAdminProductData(c *gin.Context) {
	if !h.requireProductDataHandler(c) {
		return
	}
	h.productDataHandler.UpdateProductData(c)
}

func (h *handler) UpdateAdminProductDataStatus(c *gin.Context) {
	if !h.requireProductDataHandler(c) {
		return
	}
	h.productDataHandler.UpdateProductDataStatus(c)
}

func (h *handler) DeleteAdminProductData(c *gin.Context) {
	if !h.requireProductDataHandler(c) {
		return
	}
	h.productDataHandler.DeleteProductData(c)
}

func (h *handler) requireProductDataHandler(c *gin.Context) bool {
	if h.productDataHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "product_data_repository_unavailable",
		"message": "ListingKit product data repository is not configured",
	})
	return false
}
