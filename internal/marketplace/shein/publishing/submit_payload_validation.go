package publishing

import (
	"fmt"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

// ValidateProductPublishPayload validates required SKC image fields for SHEIN publish submit payloads.
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
