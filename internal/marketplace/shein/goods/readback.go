package goods

import model "task-processor/internal/marketplace/shein/model"

// Query results are positive observations, never proof that a write had no effect.
func CorrelatesProductReadback(input model.PublishProduct, spu string, result model.ProductReadback) bool {
	if result.Product.SPUName != spu || result.CategoryID != input.CategoryID || result.ProductTypeID != input.ProductTypeID || result.BrandCode != input.BrandCode || !CorrelatesPublishedMembership(input, result.Product) {
		return false
	}
	if input.SupplierCode != "" && result.SupplierCode != input.SupplierCode {
		return false
	}
	title := ""
	for _, v := range input.Names {
		if v.Language == "en" {
			title = v.Name
		}
	}
	observed := ""
	for _, v := range result.Names {
		if v.Language == "en" {
			if observed != "" {
				return false
			}
			observed = v.Name
		}
	}
	if title == "" || title != observed {
		return false
	}
	expected := map[string]string{}
	for _, group := range input.SKCs {
		for _, sku := range group.SKUs {
			expected[sku.SupplierSKU] = group.SupplierCode
		}
	}
	for _, group := range result.Product.SKCs {
		for _, sku := range group.SKUs {
			if code := expected[sku.SupplierSKU]; code != "" && result.SKCSupplierCodes[group.SKCName] != code {
				return false
			}
		}
	}
	return true
}
