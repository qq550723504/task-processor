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
	scope := requestListScope(c)
	query := applyListQueryScope(&ProductDataQuery{
		Platform:          strings.TrimSpace(c.Query("platform")),
		Region:            strings.TrimSpace(c.Query("region")),
		ProductID:         strings.TrimSpace(c.Query("productId")),
		ParentProductID:   strings.TrimSpace(c.Query("parentProductId")),
		Title:             strings.TrimSpace(c.Query("title")),
		Brand:             strings.TrimSpace(c.Query("brand")),
		PlatformProductID: strings.TrimSpace(c.Query("platformProductId")),
	}, scope)
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")
	query.ShelfStatus = queryIntPtr(c, "shelfStatus")

	page, err := h.repo.ListProductData(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "product_data_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ProductDataHandler) GetProductData(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	product, err := h.repo.GetProductData(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeProductDataError(c, err, "product_data_get_failed")
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductDataHandler) CreateProductData(c *gin.Context) {
	var req ProductData
	if !bindAndValidateJSON(c, &req, "invalid_product_data", func(value *ProductData) {
		value.TenantID = requestTenantID(c)
	}, validateProductData) {
		return
	}
	product, err := h.repo.CreateProductData(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "product_data_create_failed", err)
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
	if !bindAndValidateJSON(c, &req, "invalid_product_data", func(value *ProductData) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validateProductData) {
		return
	}
	product, err := h.repo.UpdateProductData(requestIdentityContext(c), &req)
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
	if !bindJSON(c, &req) {
		return
	}
	product, err := h.repo.UpdateProductDataStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status)
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
	if err := h.repo.DeleteProductData(requestIdentityContext(c), requestTenantID(c), id); err != nil {
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

var writeProductDataError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrProductDataNotFound, status: http.StatusNotFound, errorCode: "product_data_not_found"},
)
