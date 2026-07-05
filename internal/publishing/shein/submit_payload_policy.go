package shein

import (
	"fmt"
	"strings"

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

func ValidatePreparedProductPublishPayloadWithSizeChartAttributes(product *sheinproduct.Product, sizeChartAttributes []PendingAttributeCandidate) error {
	if err := ValidatePreparedProductPublishPayload(product); err != nil {
		return err
	}
	return ValidateRequiredSizeChartAttributesForSubmit(product, sizeChartAttributes)
}

func ValidateRequiredSizeChartAttributesForSubmit(product *sheinproduct.Product, candidates []PendingAttributeCandidate) error {
	if product == nil || len(candidates) == 0 {
		return nil
	}
	present := make(map[int]struct{}, len(product.SizeAttributeList))
	for _, attr := range product.SizeAttributeList {
		if attr.AttributeID <= 0 || strings.TrimSpace(attr.AttributeExtraValue) == "" {
			continue
		}
		present[attr.AttributeID] = struct{}{}
	}
	missing := make([]string, 0)
	seen := map[int]struct{}{}
	for _, candidate := range candidates {
		if !candidate.Required || candidate.AttributeID <= 0 {
			continue
		}
		if _, ok := seen[candidate.AttributeID]; ok {
			continue
		}
		seen[candidate.AttributeID] = struct{}{}
		if _, ok := present[candidate.AttributeID]; ok {
			continue
		}
		missing = append(missing, firstNonEmpty(candidate.AttributeName, candidate.AttributeNameEn, candidate.Name, fmt.Sprintf("attribute_id=%d", candidate.AttributeID)))
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("SHEIN publish blocked: missing required size chart attributes: %s", strings.Join(missing, ", "))
}
