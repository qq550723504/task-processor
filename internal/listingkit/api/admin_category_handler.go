package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminCategories(c *gin.Context) {
	if !h.requireCategoryHandler(c) {
		return
	}
	h.categoryHandler.ListCategories(c)
}

func (h *handler) GetAdminCategory(c *gin.Context) {
	if !h.requireCategoryHandler(c) {
		return
	}
	h.categoryHandler.GetCategory(c)
}

func (h *handler) CreateAdminCategory(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	if !h.requireCategoryHandler(c) {
		return
	}
	h.categoryHandler.CreateCategory(c)
}

func (h *handler) UpdateAdminCategory(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	if !h.requireCategoryHandler(c) {
		return
	}
	h.categoryHandler.UpdateCategory(c)
}

func (h *handler) UpdateAdminCategoryStatus(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	if !h.requireCategoryHandler(c) {
		return
	}
	h.categoryHandler.UpdateCategoryStatus(c)
}

func (h *handler) DeleteAdminCategory(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleTaskImport) {
		return
	}
	if !h.requireCategoryHandler(c) {
		return
	}
	h.categoryHandler.DeleteCategory(c)
}

func (h *handler) requireCategoryHandler(c *gin.Context) bool {
	if h.categoryHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "category_repository_unavailable",
		"message": "ListingKit category repository is not configured",
	})
	return false
}
