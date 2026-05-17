package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProductImportMappingHandler struct {
	repo ProductImportMappingRepository
}

func NewProductImportMappingHandler(repo ProductImportMappingRepository) *ProductImportMappingHandler {
	return &ProductImportMappingHandler{repo: repo}
}

func (h *ProductImportMappingHandler) ListProductImportMappings(c *gin.Context) {
	query := ProductImportMappingQuery{
		TenantID:                requestTenantID(c),
		OwnerUserID:             requestUserID(c),
		Page:                    queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize:                queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Platform:                strings.TrimSpace(c.Query("platform")),
		Region:                  strings.TrimSpace(c.Query("region")),
		ProductID:               strings.TrimSpace(c.Query("productId")),
		ParentProductID:         strings.TrimSpace(c.Query("parentProductId")),
		SKU:                     strings.TrimSpace(c.Query("sku")),
		PlatformProductID:       strings.TrimSpace(c.Query("platformProductId")),
		PlatformParentProductID: strings.TrimSpace(c.Query("platformParentProductId")),
	}
	query.ImportTaskID = queryInt64Ptr(c, "importTaskId")
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListProductImportMappings(withRequestUserID(c.Request.Context(), requestUserID(c)), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product_import_mapping_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ProductImportMappingHandler) GetProductImportMapping(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	mapping, err := h.repo.GetProductImportMapping(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id)
	if err != nil {
		writeProductImportMappingError(c, err, "product_import_mapping_get_failed")
		return
	}
	c.JSON(http.StatusOK, mapping)
}

func (h *ProductImportMappingHandler) CreateProductImportMapping(c *gin.Context) {
	var req ProductImportMapping
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateProductImportMapping(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_import_mapping", "message": err.Error()})
		return
	}
	mapping, err := h.repo.CreateProductImportMapping(withRequestUserID(c.Request.Context(), requestUserID(c)), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product_import_mapping_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mapping)
}

func (h *ProductImportMappingHandler) UpdateProductImportMapping(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req ProductImportMapping
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateProductImportMapping(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_import_mapping", "message": err.Error()})
		return
	}
	mapping, err := h.repo.UpdateProductImportMapping(withRequestUserID(c.Request.Context(), requestUserID(c)), &req)
	if err != nil {
		writeProductImportMappingError(c, err, "product_import_mapping_update_failed")
		return
	}
	c.JSON(http.StatusOK, mapping)
}

func (h *ProductImportMappingHandler) UpdateProductImportMappingStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int16  `json:"status"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	mapping, err := h.repo.UpdateProductImportMappingStatus(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeProductImportMappingError(c, err, "product_import_mapping_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, mapping)
}

func (h *ProductImportMappingHandler) DeleteProductImportMapping(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteProductImportMapping(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id); err != nil {
		writeProductImportMappingError(c, err, "product_import_mapping_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateProductImportMapping(mapping *ProductImportMapping) error {
	switch {
	case mapping.TenantID <= 0:
		return errors.New("tenant id is required")
	case mapping.ImportTaskID <= 0:
		return errors.New("importTaskId is required")
	case mapping.StoreID <= 0:
		return errors.New("storeId is required")
	case strings.TrimSpace(mapping.Platform) == "":
		return errors.New("platform is required")
	case strings.TrimSpace(mapping.Region) == "":
		return errors.New("region is required")
	case strings.TrimSpace(mapping.ProductID) == "":
		return errors.New("productId is required")
	}
	return nil
}

func writeProductImportMappingError(c *gin.Context, err error, code string) {
	if errors.Is(err, ErrProductImportMappingNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_import_mapping_not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
}
