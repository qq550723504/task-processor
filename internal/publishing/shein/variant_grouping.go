package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

type variantGroup struct {
	skcName      string
	saleName     string
	supplierCode string
	mainImageURL string
	attributes   map[string]string
	skus         []common.Variant
}

func buildVariantGroups(baseTitle string, variants []common.Variant, images *common.ImageSet, resolution *SaleAttributeResolution) []variantGroup {
	if len(variants) == 0 {
		return nil
	}

	groupingKey := ""
	if resolution != nil {
		groupingKey = strings.TrimSpace(resolution.PrimarySourceDimension)
	}

	groupOrder := make([]string, 0, len(variants))
	grouped := make(map[string][]common.Variant, len(variants))
	for _, variant := range variants {
		groupKey := buildVariantGroupKey(groupingKey, variant)
		if _, ok := grouped[groupKey]; !ok {
			groupOrder = append(groupOrder, groupKey)
		}
		grouped[groupKey] = append(grouped[groupKey], variant)
	}

	result := make([]variantGroup, 0, len(groupOrder))
	for _, groupKey := range groupOrder {
		groupVariants := grouped[groupKey]
		if len(groupVariants) == 0 {
			continue
		}
		representative := groupVariants[0]
		groupAttributes := commonAttributes(groupVariants)
		skcName := resolveSKCName(baseTitle, groupingKey, groupAttributes, representative)
		mainImageURL := common.FirstNonEmpty(representative.Image, images.MainImage)
		result = append(result, variantGroup{
			skcName:      skcName,
			saleName:     skcName,
			supplierCode: representative.SKU,
			mainImageURL: mainImageURL,
			attributes:   groupAttributes,
			skus:         append([]common.Variant(nil), groupVariants...),
		})
	}
	return result
}

func buildVariantGroupKey(groupingKey string, variant common.Variant) string {
	if groupingKey == "" {
		return "__default__"
	}
	value := strings.TrimSpace(lookupAttributeValue(variant.Attributes, groupingKey))
	if value == "" {
		return "__missing__:" + strings.TrimSpace(variant.SKU)
	}
	return normalizeText(value)
}

func commonAttributes(variants []common.Variant) map[string]string {
	if len(variants) == 0 {
		return nil
	}
	result := common.CloneMap(variants[0].Attributes)
	for key, value := range result {
		for _, variant := range variants[1:] {
			if normalizeText(variant.Attributes[key]) != normalizeText(value) {
				delete(result, key)
				break
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func resolveSKCName(baseTitle string, groupingKey string, attributes map[string]string, representative common.Variant) string {
	groupName := ""
	if groupingKey != "" {
		if value := strings.TrimSpace(lookupAttributeValue(attributes, groupingKey)); value != "" {
			groupName = value
		} else if value := strings.TrimSpace(lookupAttributeValue(representative.Attributes, groupingKey)); value != "" {
			groupName = value
		}
	}
	if groupName != "" {
		baseTitle = strings.TrimSpace(baseTitle)
		if baseTitle != "" && !strings.EqualFold(normalizeText(baseTitle), normalizeText(groupName)) {
			return baseTitle + " - " + groupName
		}
		return groupName
	}
	return common.FirstNonEmpty(representative.SKU, "DEFAULT-001")
}
