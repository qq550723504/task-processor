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
	scope := requestListScope(c)
	query := applyListQueryScope(&CategoryQuery{
		Name: strings.TrimSpace(c.Query("name")),
		Code: strings.TrimSpace(c.Query("code")),
	}, scope)
	var ok bool
	query.ParentID, ok = queryInt64PtrStrict(c, "parentId", "invalid_parent_id")
	if !ok {
		return
	}
	query.Level, ok = queryIntPtrStrict(c, "level", "invalid_level")
	if !ok {
		return
	}
	query.Status, ok = queryInt16PtrStrict(c, "status", "invalid_status")
	if !ok {
		return
	}

	items, err := h.repo.ListCategories(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "category_list_failed", err)
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
	if !bindAndValidateJSON(c, &req, "invalid_category", func(value *Category) {
		value.TenantID = requestTenantID(c)
	}, validateCategory) {
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
	if !bindAndValidateJSON(c, &req, "invalid_category", func(value *Category) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validateCategory) {
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
	if !bindJSON(c, &req) {
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

var writeCategoryError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrCategoryNotFound, status: http.StatusNotFound, errorCode: "category_not_found"},
	handlerErrorRule{match: ErrCategoryHasChildren, status: http.StatusConflict, errorCode: "category_has_children"},
)
