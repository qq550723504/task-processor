package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProductDataHandler struct{ repo ProductDataRepository }

func NewProductDataHandler(repo ProductDataRepository) *ProductDataHandler {
	return &ProductDataHandler{repo: repo}
}

func (h *ProductDataHandler) ListProductData(c *gin.Context) {
	query := ProductDataQuery{
		TenantID:          requestTenantID(c),
		Page:              queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize:          queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Platform:          strings.TrimSpace(c.Query("platform")),
		Region:            strings.TrimSpace(c.Query("region")),
		ProductID:         strings.TrimSpace(c.Query("productId")),
		ParentProductID:   strings.TrimSpace(c.Query("parentProductId")),
		Title:             strings.TrimSpace(c.Query("title")),
		Brand:             strings.TrimSpace(c.Query("brand")),
		PlatformProductID: strings.TrimSpace(c.Query("platformProductId")),
	}
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")
	query.ShelfStatus = queryIntPtr(c, "shelfStatus")

	page, err := h.repo.ListProductData(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product_data_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ProductDataHandler) GetProductData(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	product, err := h.repo.GetProductData(c.Request.Context(), requestTenantID(c), id)
	if err != nil {
		writeProductDataError(c, err, "product_data_get_failed")
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductDataHandler) CreateProductData(c *gin.Context) {
	var req ProductData
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateProductData(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_data", "message": err.Error()})
		return
	}
	product, err := h.repo.CreateProductData(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product_data_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, product)
}

func (h *ProductDataHandler) UpdateProductData(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req ProductData
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateProductData(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_data", "message": err.Error()})
		return
	}
	product, err := h.repo.UpdateProductData(c.Request.Context(), &req)
	if err != nil {
		writeProductDataError(c, err, "product_data_update_failed")
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductDataHandler) UpdateProductDataStatus(c *gin.Context) {
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
	product, err := h.repo.UpdateProductDataStatus(c.Request.Context(), requestTenantID(c), id, req.Status)
	if err != nil {
		writeProductDataError(c, err, "product_data_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductDataHandler) DeleteProductData(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteProductData(c.Request.Context(), requestTenantID(c), id); err != nil {
		writeProductDataError(c, err, "product_data_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateProductData(product *ProductData) error {
	switch {
	case product.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(product.ProductID) == "":
		return errors.New("productId is required")
	}
	return nil
}

func writeProductDataError(c *gin.Context, err error, code string) {
	if errors.Is(err, ErrProductDataNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_data_not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
}
