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
		if shouldSplitVariantGroupPerVariant(groupVariants, resolution) {
			for _, groupVariant := range groupVariants {
				groupAttributes := common.CloneMap(groupVariant.Attributes)
				skcName := resolveSKCNameWithResolution(baseTitle, groupingKey, groupAttributes, groupVariant, resolution)
				mainImageURL := common.FirstNonEmpty(groupVariant.Image, images.MainImage)
				result = append(result, variantGroup{
					skcName:      skcName,
					saleName:     skcName,
					supplierCode: groupVariant.SKU,
					mainImageURL: mainImageURL,
					attributes:   groupAttributes,
					skus:         []common.Variant{groupVariant},
				})
			}
			continue
		}
		representative := groupVariants[0]
		groupAttributes := commonAttributes(groupVariants)
		skcName := resolveSKCNameWithResolution(baseTitle, groupingKey, groupAttributes, representative, resolution)
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

func shouldSplitVariantGroupPerVariant(groupVariants []common.Variant, resolution *SaleAttributeResolution) bool {
	if len(groupVariants) <= 1 || resolution == nil {
		return false
	}
	if strings.TrimSpace(resolution.SecondarySourceDimension) == "" {
		return false
	}
	if resolution.SecondaryAttributeID > 0 {
		return false
	}
	if hasUsableSecondaryCandidateForSourceDimension(resolution) {
		return false
	}
	secondaryValues := make(map[string]struct{}, len(groupVariants))
	for _, groupVariant := range groupVariants {
		value := strings.TrimSpace(lookupAttributeValue(groupVariant.Attributes, resolution.SecondarySourceDimension))
		if value == "" {
			continue
		}
		secondaryValues[normalizeText(value)] = struct{}{}
	}
	return len(secondaryValues) > 1
}

func hasUsableSecondaryCandidateForSourceDimension(resolution *SaleAttributeResolution) bool {
	if resolution == nil {
		return false
	}
	secondarySourceDimension := strings.TrimSpace(resolution.SecondarySourceDimension)
	if secondarySourceDimension == "" {
		return false
	}
	for _, candidate := range resolution.Candidates {
		if candidate.AttributeID == 0 || candidate.SKCScope {
			continue
		}
		if candidate.SelectedScope == "secondary" {
			return true
		}
		if saleDimensionMatches(candidate.SourceDimension, secondarySourceDimension) ||
			saleDimensionMatches(candidate.Name, secondarySourceDimension) {
			return true
		}
	}
	for _, option := range resolution.TemplateOptions {
		if option.AttributeID == 0 || option.SKCScope {
			continue
		}
		if saleDimensionMatches(option.Name, secondarySourceDimension) ||
			saleDimensionMatches(option.NameEn, secondarySourceDimension) {
			return true
		}
	}
	return false
}

func saleDimensionMatches(left string, right string) bool {
	return normalizeSaleDimension(left) != "" && normalizeSaleDimension(left) == normalizeSaleDimension(right)
}

func normalizeSaleDimension(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "color", "colour", "颜色", "颜色分类":
		return "color"
	case "size", "尺码", "尺寸", "规格":
		return "size"
	case "quantity", "count", "件数", "数量":
		return "quantity"
	case "style", "style type", "款式", "类型":
		return "style"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func resolveSKCNameWithResolution(baseTitle string, groupingKey string, attributes map[string]string, representative common.Variant, resolution *SaleAttributeResolution) string {
	baseTitle = strings.TrimSpace(baseTitle)
	if baseTitle != "" {
		baseTitle = trimShortTitle(buildSKCBaseTitle(baseTitle, nil, baseTitle), 70, 8)
	}
	groupName := resolveSKCGroupName(groupingKey, attributes, representative, resolution)
	if groupName != "" {
		baseTitle = strings.TrimSpace(baseTitle)
		if baseTitle != "" && !strings.EqualFold(normalizeText(baseTitle), normalizeText(groupName)) {
			return baseTitle + " - " + groupName
		}
		return groupName
	}
	return common.FirstNonEmpty(representative.SKU, "DEFAULT-001")
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
	baseTitle = strings.TrimSpace(baseTitle)
	if baseTitle != "" {
		baseTitle = trimShortTitle(buildSKCBaseTitle(baseTitle, nil, baseTitle), 70, 8)
	}
	groupName := ""
	if groupingKey != "" {
		groupName = resolveSKCGroupName(groupingKey, attributes, representative, nil)
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

func resolveSKCGroupName(groupingKey string, attributes map[string]string, representative common.Variant, resolution *SaleAttributeResolution) string {
	if groupingKey == "" {
		return ""
	}
	rawValue := strings.TrimSpace(lookupAttributeValue(attributes, groupingKey))
	if rawValue == "" {
		rawValue = strings.TrimSpace(lookupAttributeValue(representative.Attributes, groupingKey))
	}
	if rawValue == "" {
		return ""
	}
	if assigned, ok := resolveSaleAttributeValueAssignment(effectiveSKCValueAssignments(resolution), rawValue); ok {
		if value := strings.TrimSpace(assigned.Value); value != "" {
			return value
		}
	}
	return rawValue
}
