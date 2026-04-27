package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

func buildDerivedAttributeInputs(pkg *Package) []common.Attribute {
	if pkg == nil {
		return nil
	}
	result := make([]common.Attribute, 0, 12)
	appendValue := func(name string, value string) {
		value = strings.TrimSpace(value)
		if strings.TrimSpace(name) == "" || value == "" {
			return
		}
		result = append(result, common.Attribute{Name: name, Value: value})
	}

	appendValue("Product Title", firstNonEmpty(pkg.SpuName, pkg.ProductNameEn, pkg.ProductNameMulti))
	appendValue("Category Name", pkg.CategoryName)
	if len(pkg.CategoryPath) > 0 {
		appendValue("Category Path", strings.Join(pkg.CategoryPath, " > "))
	}
	appendValue("Description", pkg.Description)

	if pkg.RequestDraft == nil || len(pkg.RequestDraft.SKCList) == 0 || len(pkg.RequestDraft.SKCList[0].SKUList) == 0 {
		return dedupeAttributeInputs(result)
	}
	sku := pkg.RequestDraft.SKCList[0].SKUList[0]
	appendValue("Length (cm)", sku.Length)
	appendValue("Width (cm)", sku.Width)
	appendValue("Height (cm)", sku.Height)
	appendValue("Weight", common.FormatFloat(sku.Weight))
	appendValue("Weight ("+strings.ToLower(strings.TrimSpace(sku.WeightUnit))+")", common.FormatFloat(sku.Weight))
	return dedupeAttributeInputs(result)
}

func dedupeAttributeInputs(items []common.Attribute) []common.Attribute {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]common.Attribute, 0, len(items))
	for _, item := range items {
		key := normalizeText(item.Name) + "\x00" + strings.TrimSpace(item.Value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
