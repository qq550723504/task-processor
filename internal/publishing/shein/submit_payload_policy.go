package shein

import (
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinproduct "task-processor/internal/shein/api/product"
)

func DeriveSubmitProductSupplierCode(product *sheinproduct.Product) string {
	if product == nil {
		return ""
	}
	supplierSKUs := make([]string, 0)
	for _, skc := range product.SKCList {
		for _, sku := range skc.SKUS {
			supplierSKUs = append(supplierSKUs, sku.SupplierSKU)
		}
	}
	return sheinmarketpub.DeriveSubmitSupplierCode(product.SupplierCode, supplierSKUs)
}

func ValidateProductPublishPayload(product *sheinproduct.Product) error {
	return sheinmarketpub.ValidateProductPublishPayload(product)
}

// ValidatePreparedProductPublishPayload validates a submit product after submit normalization has run.
func ValidatePreparedProductPublishPayload(product *sheinproduct.Product) error {
	return sheinmarketpub.ValidatePreparedProductPublishPayload(product)
}
