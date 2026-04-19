package listingkit

import (
	"fmt"
	"strings"
)

type RevisionFieldError struct {
	FieldPath string `json:"field_path,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

type RevisionValidationError struct {
	Fields []RevisionFieldError `json:"fields,omitempty"`
}

func (e *RevisionValidationError) Error() string {
	if e == nil || len(e.Fields) == 0 {
		return ErrInvalidRevisionRequest.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidRevisionRequest.Error(), e.Fields[0].Message)
}

func (e *RevisionValidationError) Unwrap() error {
	return ErrInvalidRevisionRequest
}

func validateApplyRevisionRequest(req *ApplyRevisionRequest) error {
	if req == nil {
		return ErrInvalidRevisionRequest
	}

	var fieldErrors []RevisionFieldError
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	switch platform {
	case "shein":
		if req.Shein == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "shein",
				Code:      "required",
				Message:   "缺少 shein revision payload",
			})
			break
		}
		fieldErrors = append(fieldErrors, validateSheinRevisionInput(req.Shein)...)
	case "amazon":
		if req.Amazon == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "amazon",
				Code:      "required",
				Message:   "缺少 amazon revision payload",
			})
		}
	case "temu":
		if req.Temu == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "temu",
				Code:      "required",
				Message:   "缺少 temu revision payload",
			})
		}
	case "walmart":
		if req.Walmart == nil {
			fieldErrors = append(fieldErrors, RevisionFieldError{
				FieldPath: "walmart",
				Code:      "required",
				Message:   "缺少 walmart revision payload",
			})
		}
	}

	if len(fieldErrors) == 0 {
		return nil
	}
	return &RevisionValidationError{Fields: fieldErrors}
}

func validateSheinRevisionInput(req *SheinRevisionInput) []RevisionFieldError {
	if req == nil {
		return nil
	}
	var fieldErrors []RevisionFieldError

	if req.CategoryID != nil && *req.CategoryID <= 0 {
		fieldErrors = append(fieldErrors, newRevisionFieldError("shein.category_id", "invalid_value", "category_id 必须大于 0"))
	}
	if req.ProductTypeID != nil && *req.ProductTypeID <= 0 {
		fieldErrors = append(fieldErrors, newRevisionFieldError("shein.product_type_id", "invalid_value", "product_type_id 必须大于 0"))
	}
	if req.TopCategoryID != nil && *req.TopCategoryID <= 0 {
		fieldErrors = append(fieldErrors, newRevisionFieldError("shein.top_category_id", "invalid_value", "top_category_id 必须大于 0"))
	}
	if req.Images != nil && firstNonEmpty(req.Images.MainImage, req.Images.WhiteBgImage) == "" && len(req.Images.Gallery) == 0 {
		fieldErrors = append(fieldErrors, newRevisionFieldError("shein.images", "invalid_value", "images 至少需要包含主图、白底图或 gallery"))
	}
	if req.CategoryResolution != nil {
		if req.CategoryResolution.CategoryID != nil && *req.CategoryResolution.CategoryID <= 0 {
			fieldErrors = append(fieldErrors, newRevisionFieldError("shein.category_resolution.category_id", "invalid_value", "category_resolution.category_id 必须大于 0"))
		}
		if req.CategoryResolution.ProductTypeID != nil && *req.CategoryResolution.ProductTypeID <= 0 {
			fieldErrors = append(fieldErrors, newRevisionFieldError("shein.category_resolution.product_type_id", "invalid_value", "category_resolution.product_type_id 必须大于 0"))
		}
	}
	if req.AttributeResolution != nil {
		for i, attr := range req.AttributeResolution.ResolvedAttributes {
			if attr.AttributeID <= 0 {
				fieldErrors = append(fieldErrors, newRevisionFieldError(
					fmt.Sprintf("shein.attribute_resolution.resolved_attributes[%d].attribute_id", i),
					"required",
					"resolved attribute 需要有效的 attribute_id",
				))
			}
		}
	}
	if req.SaleAttributeResolution != nil {
		if req.SaleAttributeResolution.PrimaryAttributeID != nil && *req.SaleAttributeResolution.PrimaryAttributeID <= 0 {
			fieldErrors = append(fieldErrors, newRevisionFieldError("shein.sale_attribute_resolution.primary_attribute_id", "invalid_value", "primary_attribute_id 必须大于 0"))
		}
		if req.SaleAttributeResolution.SecondaryAttributeID != nil && *req.SaleAttributeResolution.SecondaryAttributeID <= 0 {
			fieldErrors = append(fieldErrors, newRevisionFieldError("shein.sale_attribute_resolution.secondary_attribute_id", "invalid_value", "secondary_attribute_id 必须大于 0"))
		}
	}
	for i, patch := range req.SKCPatches {
		if strings.TrimSpace(patch.SupplierCode) == "" {
			fieldErrors = append(fieldErrors, newRevisionFieldError(fmt.Sprintf("shein.skc_patches[%d].supplier_code", i), "required", "skc patch 需要 supplier_code"))
		}
		for j, skuPatch := range patch.SKUPatches {
			if strings.TrimSpace(skuPatch.SupplierSKU) == "" {
				fieldErrors = append(fieldErrors, newRevisionFieldError(fmt.Sprintf("shein.skc_patches[%d].sku_patches[%d].supplier_sku", i, j), "required", "sku patch 需要 supplier_sku"))
			}
			if skuPatch.StockCount != nil && *skuPatch.StockCount < 0 {
				fieldErrors = append(fieldErrors, newRevisionFieldError(fmt.Sprintf("shein.skc_patches[%d].sku_patches[%d].stock_count", i, j), "invalid_value", "stock_count 不能小于 0"))
			}
		}
	}
	return fieldErrors
}

func newRevisionFieldError(fieldPath, code, message string) RevisionFieldError {
	return RevisionFieldError{
		FieldPath: fieldPath,
		Code:      code,
		Message:   message,
	}
}
