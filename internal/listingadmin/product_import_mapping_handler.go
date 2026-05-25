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
	scope := requestListScope(c)
	query := ProductImportMappingQuery{
		TenantID:                scope.TenantID,
		OwnerUserID:             scope.OwnerUserID,
		Page:                    scope.Page,
		PageSize:                scope.PageSize,
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

	page, err := h.repo.ListProductImportMappings(requestIdentityContext(c), query)
	if err != nil {
		writeInternalHandlerError(c, "product_import_mapping_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ProductImportMappingHandler) GetProductImportMapping(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	mapping, err := h.repo.GetProductImportMapping(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeProductImportMappingError(c, err, "product_import_mapping_get_failed")
		return
	}
	c.JSON(http.StatusOK, mapping)
}

func (h *ProductImportMappingHandler) CreateProductImportMapping(c *gin.Context) {
	var req ProductImportMapping
	if !bindJSON(c, &req) {
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateProductImportMapping(&req); err != nil {
		writeValidationError(c, "invalid_product_import_mapping", err)
		return
	}
	mapping, err := h.repo.CreateProductImportMapping(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "product_import_mapping_create_failed", err)
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
	if !bindJSON(c, &req) {
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateProductImportMapping(&req); err != nil {
		writeValidationError(c, "invalid_product_import_mapping", err)
		return
	}
	mapping, err := h.repo.UpdateProductImportMapping(requestIdentityContext(c), &req)
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
	if !bindJSON(c, &req) {
		return
	}
	mapping, err := h.repo.UpdateProductImportMappingStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
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
	if err := h.repo.DeleteProductImportMapping(requestIdentityContext(c), requestTenantID(c), id); err != nil {
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
	writeMappedHandlerError(c, err, code,
		handlerErrorRule{match: ErrProductImportMappingNotFound, status: http.StatusNotFound, errorCode: "product_import_mapping_not_found"},
	)
}
