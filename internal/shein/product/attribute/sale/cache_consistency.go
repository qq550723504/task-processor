package sale

import (
	"strings"

	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func (h *SaleAttributeHandler) cachedSaleSpecMatchesCurrentVariants(currentVariants *[]model.Product, cached sheinattr.ResultSaleAttribute) bool {
	if currentVariants == nil {
		return len(cached.Variants) == 0
	}

	expected := make(map[string]struct{}, len(*currentVariants))
	for _, variant := range *currentVariants {
		asin := strings.TrimSpace(variant.Asin)
		if asin == "" {
			continue
		}
		expected[asin] = struct{}{}
	}

	if len(expected) == 0 {
		return len(cached.Variants) == 0
	}

	actual := make(map[string]struct{}, len(cached.Variants))
	for _, variant := range cached.Variants {
		asin := strings.TrimSpace(variant.ASIN)
		if asin == "" {
			return false
		}
		if _, ok := expected[asin]; !ok {
			return false
		}
		actual[asin] = struct{}{}
	}

	return len(actual) == len(expected)
}
