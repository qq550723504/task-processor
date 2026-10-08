package goods

import (
	"fmt"
	model "task-processor/internal/marketplace/shein/model"
)

// LinkedRuleGroups preserves each actual attribute combination. Official
// requests take batches of at most ten groups; callers must not truncate them.
func LinkedRuleGroups(product model.PublishProduct) []model.LinkedRuleGroup {
	project := func(values ...[]model.AttributeValue) []model.AttributeValue {
		result := []model.AttributeValue{}
		for _, list := range values {
			for _, value := range list {
				if value.AttributeID <= 0 {
					continue
				}
				copy := model.AttributeValue{AttributeID: value.AttributeID}
				if value.AttributeValueID != nil && *value.AttributeValueID > 0 {
					id := *value.AttributeValueID
					copy.AttributeValueID = &id
				}
				result = append(result, copy)
			}
		}
		return result
	}
	groups := []model.LinkedRuleGroup{{ID: "product", CategoryID: product.CategoryID, ProductTypeID: product.ProductTypeID, Attributes: project(product.Attributes)}}
	for i, skc := range product.SKCs {
		for j, sku := range skc.SKUs {
			groups = append(groups, model.LinkedRuleGroup{ID: fmt.Sprintf("sku-%d-%d", i, j), CategoryID: product.CategoryID, ProductTypeID: product.ProductTypeID, Attributes: project(product.Attributes, []model.AttributeValue{skc.SaleAttribute}, sku.SaleAttributes, sku.Attributes)})
		}
	}
	return groups
}

func ProductTypeForCategory(categories []model.Category, categoryID int64) (int64, bool) {
	category, ok := findOfficialCategory(categories, categoryID)
	return category.ProductTypeID, ok && category.Leaf != nil && *category.Leaf && category.ProductTypeID > 0
}
