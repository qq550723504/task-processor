package shein

import (
	"fmt"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

func DeriveSubmitProductSupplierCode(product *sheinproduct.Product) string {
	if product == nil {
		return ""
	}
	if value := strings.TrimSpace(product.SupplierCode); value != "" && !looksLikeRawBaseSupplierCode(value) {
		return value
	}
	for _, skc := range product.SKCList {
		for _, sku := range skc.SKUS {
			if value := deriveSubmitSupplierCodeFromSKU(sku.SupplierSKU); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(product.SupplierCode)
}

func deriveSubmitSupplierCodeFromSKU(supplierSKU string) string {
	supplierSKU = strings.TrimSpace(strings.ToUpper(supplierSKU))
	if supplierSKU == "" {
		return ""
	}
	parts := strings.Split(supplierSKU, "-")
	if len(parts) < 2 {
		return supplierSKU
	}
	styleSuffix := normalizeSubmitStyleSuffix(parts[len(parts)-1])
	if styleSuffix == "" {
		return supplierSKU
	}
	baseSKU := strings.TrimSpace(parts[0])
	if baseSKU == "" {
		return ""
	}
	return baseSKU + "-" + styleSuffix
}

func normalizeSubmitStyleSuffix(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
}

func looksLikeRawBaseSupplierCode(value string) bool {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return false
	}
	parts := strings.Split(value, "-")
	return len(parts) == 1
}

func ValidateProductPublishPayload(product *sheinproduct.Product) error {
	if product == nil {
		return fmt.Errorf("SHEIN publish payload is empty")
	}
	for skcIndex, skc := range product.SKCList {
		if len(skc.ImageInfo.ImageInfoList) == 0 {
			return fmt.Errorf("SHEIN publish blocked: SKC[%d] has no images", skcIndex)
		}
		hasSquare := false
		hasColorBlock := false
		for _, image := range skc.ImageInfo.ImageInfoList {
			switch image.ImageType {
			case 5:
				hasSquare = true
			case 6:
				hasColorBlock = true
			}
		}
		if !hasSquare {
			return fmt.Errorf("SHEIN publish blocked: SKC[%d] is missing required square image", skcIndex)
		}
		if !hasColorBlock {
			return fmt.Errorf("SHEIN publish blocked: SKC[%d] is missing required color block image", skcIndex)
		}
	}
	return nil
}

// ValidatePreparedProductPublishPayload validates a submit product after submit normalization has run.
func ValidatePreparedProductPublishPayload(product *sheinproduct.Product) error {
	if err := ValidateProductPublishPayload(product); err != nil {
		return err
	}
	for skcIndex, skc := range product.SKCList {
		for skuIndex, sku := range skc.SKUS {
			if sku.QuantityInfo == nil || sku.QuantityInfo.Quantity == nil || sku.QuantityInfo.QuantityType == nil || sku.QuantityInfo.QuantityUnit == nil {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing quantity_info", skcIndex, skuIndex)
			}
			if sku.PackageType == 0 {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing package_type", skcIndex, skuIndex)
			}
			if len(sku.StockInfoList) == 0 {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing stock_info_list", skcIndex, skuIndex)
			}
			if strings.TrimSpace(sku.Length) == "" ||
				strings.TrimSpace(sku.Width) == "" ||
				strings.TrimSpace(sku.Height) == "" ||
				strings.TrimSpace(sku.LengthUnit) == "" {
				return fmt.Errorf("SHEIN publish blocked: SKC[%d] SKU[%d] is missing package dimensions", skcIndex, skuIndex)
			}
		}
	}
	return nil
}
