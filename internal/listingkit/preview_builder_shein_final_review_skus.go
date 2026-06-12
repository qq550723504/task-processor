package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinFinalReviewSKUs(draft *sheinpub.RequestDraft) []SheinFinalReviewSKU {
	if draft == nil {
		return nil
	}
	out := []SheinFinalReviewSKU{}
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			out = append(out, buildSheinFinalReviewSKU(skc.SupplierCode, sku))
		}
	}
	return out
}

func buildSheinFinalReviewSKU(supplierCode string, sku SheinSKUDraft) SheinFinalReviewSKU {
	item := SheinFinalReviewSKU{
		SupplierCode: supplierCode,
		SupplierSKU:  sku.SupplierSKU,
		Price:        parseMoney(sku.BasePrice),
		Currency:     sku.Currency,
		Stock:        sku.StockCount,
		Weight:       sku.Weight,
	}
	for _, attr := range sku.SaleAttributes {
		switch normalizeSheinFinalReviewAttributeName(attr.Name) {
		case "color":
			item.Color = attr.Value
		case "size":
			item.Size = attr.Value
		}
	}
	return item
}

func normalizeSheinFinalReviewAttributeName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "color", "颜色":
		return "color"
	case "size", "尺码", "尺寸":
		return "size"
	default:
		return ""
	}
}
