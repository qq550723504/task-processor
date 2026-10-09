package goods

import (
	"strings"
	model "task-processor/internal/marketplace/shein/model"
	"unicode"
	"unicode/utf8"
)

// CorrelatesPublishedMembership is the reusable deterministic part of the
// official response contract. The transport still proves business success and
// rejects filters / blocking messages before producing this typed result.
func CorrelatesPublishedMembership(input model.PublishProduct, result model.PublishResult) bool {
	bounded := func(value string, maximum int) bool {
		return value != "" && len(value) <= maximum && value == strings.TrimSpace(value) && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
	}
	if !bounded(result.SPUName, 128) || len(input.SKCs) < 1 || len(input.SKCs) > 40 || len(result.SKCs) != len(input.SKCs) || result.Version != "" && !bounded(result.Version, 128) || result.TraceID != "" && !bounded(result.TraceID, 128) {
		return false
	}
	expected := map[string]int{}
	for i, skc := range input.SKCs {
		if len(skc.SKUs) < 1 || len(skc.SKUs) > 400 {
			return false
		}
		for _, sku := range skc.SKUs {
			if !bounded(sku.SupplierSKU, 800) {
				return false
			}
			if _, duplicate := expected[sku.SupplierSKU]; duplicate {
				return false
			}
			expected[sku.SupplierSKU] = i
		}
	}
	seenSKUs, seenCodes, seenSKCs, seenGroups := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[int]bool{}
	for _, skc := range result.SKCs {
		if !bounded(skc.SKCName, 128) || seenSKCs[skc.SKCName] || len(skc.SKUs) == 0 || len(skc.SKUs) > 400 {
			return false
		}
		seenSKCs[skc.SKCName] = true
		group := -1
		for _, sku := range skc.SKUs {
			i, exists := expected[sku.SupplierSKU]
			if !exists || seenSKUs[sku.SupplierSKU] || !bounded(sku.SKUCode, 128) || seenCodes[sku.SKUCode] || group >= 0 && group != i {
				return false
			}
			group = i
			seenSKUs[sku.SupplierSKU] = true
			seenCodes[sku.SKUCode] = true
		}
		if seenGroups[group] || len(skc.SKUs) != len(input.SKCs[group].SKUs) {
			return false
		}
		seenGroups[group] = true
	}
	return len(seenSKUs) == len(expected)
}
