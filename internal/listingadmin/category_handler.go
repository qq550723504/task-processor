package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct{ repo CategoryRepository }

func NewCategoryHandler(repo CategoryRepository) *CategoryHandler {
	return &CategoryHandler{repo: repo}
}

func (h *CategoryHandler) ListCategories(c *gin.Context) {
	query := CategoryQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestScopedOwnerUserID(c),
		Name:        strings.TrimSpace(c.Query("name")),
		Code:        strings.TrimSpace(c.Query("code")),
	}
	query.ParentID = queryInt64Ptr(c, "parentId")
	query.Level = queryIntPtr(c, "level")
	query.Status = queryInt16Ptr(c, "status")

	items, err := h.repo.ListCategories(requestIdentityContext(c), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "category_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *CategoryHandler) GetCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	category, err := h.repo.GetCategory(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeCategoryError(c, err, "category_get_failed")
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req Category
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateCategory(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_category", "message": err.Error()})
		return
	}
	category, err := h.repo.CreateCategory(requestIdentityContext(c), &req)
	if err != nil {
		writeCategoryError(c, err, "category_create_failed")
		return
	}
	c.JSON(http.StatusCreated, category)
}

func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req Category
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateCategory(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_category", "message": err.Error()})
		return
	}
	category, err := h.repo.UpdateCategory(requestIdentityContext(c), &req)
	if err != nil {
		writeCategoryError(c, err, "category_update_failed")
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) UpdateCategoryStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int16 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	category, err := h.repo.UpdateCategoryStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status)
	if err != nil {
		writeCategoryError(c, err, "category_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteCategory(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeCategoryError(c, err, "category_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateCategory(category *Category) error {
	switch {
	case category.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(category.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(category.Code) == "":
		return errors.New("code is required")
	case category.Level <= 0:
		return errors.New("level is required")
	}
	return nil
}

func writeCategoryError(c *gin.Context, err error, code string) {
	switch {
	case errors.Is(err, ErrCategoryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "category_not_found", "message": err.Error()})
	case errors.Is(err, ErrCategoryHasChildren):
		c.JSON(http.StatusConflict, gin.H{"error": "category_has_children", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
	}
}
