package shein

import (
	"fmt"
	"sort"
	"strconv"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func buildSaleAttributeTemplateOptions(attributes []sheinattribute.AttributeInfo) []SaleAttributeTemplateOption {
	if len(attributes) == 0 {
		return nil
	}
	options := make([]SaleAttributeTemplateOption, 0, len(attributes))
	for _, attribute := range attributes {
		option := SaleAttributeTemplateOption{
			AttributeID: attribute.AttributeID,
			Name:        attribute.AttributeName,
			NameEn:      attribute.AttributeNameEn,
			Required:    attribute.AttributeIsShow == 1,
			Important:   attribute.AttributeLabel == 1,
		}
		if attribute.SKCScope != nil {
			option.SKCScope = *attribute.SKCScope
		}
		if len(attribute.AttributeValueInfoList) > 0 {
			option.AttributeValueList = make([]AttributeValueCandidate, 0, len(attribute.AttributeValueInfoList))
			for _, value := range attribute.AttributeValueInfoList {
				if value.AttributeValueID <= 0 {
					continue
				}
				option.AttributeValueList = append(option.AttributeValueList, AttributeValueCandidate{
					AttributeValueID: value.AttributeValueID,
					Value:            value.AttributeValue,
					ValueEn:          value.AttributeValueEn,
				})
			}
		}
		options = append(options, option)
	}
	return options
}

func filterSaleScopeAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope) {
			result = append(result, attr)
			continue
		}
		switch normalizeText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)) {
		case "color", "colour", "size", "style", "pattern", "capacity", "type", "model", "set", "颜色", "颜色分类", "尺码", "尺寸", "规格", "容量", "款式", "类型", "型号", "套装":
			result = append(result, attr)
		}
	}
	return result
}

func orderSaleScopeAttributes(attributes []sheinattribute.AttributeInfo, orderedIDs []int) []sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return attributes
	}
	if len(orderedIDs) == 0 {
		ordered := append([]sheinattribute.AttributeInfo(nil), attributes...)
		sort.SliceStable(ordered, func(i, j int) bool {
			left := isPrimarySaleTemplateAttribute(ordered[i])
			right := isPrimarySaleTemplateAttribute(ordered[j])
			if left != right {
				return left
			}
			return false
		})
		return ordered
	}
	byID := make(map[int]sheinattribute.AttributeInfo, len(attributes))
	for _, attr := range attributes {
		byID[attr.AttributeID] = attr
	}
	ordered := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	seen := make(map[int]struct{}, len(attributes))
	for _, id := range orderedIDs {
		attr, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, attr)
		seen[id] = struct{}{}
	}
	for _, attr := range attributes {
		if _, ok := seen[attr.AttributeID]; ok {
			continue
		}
		ordered = append(ordered, attr)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left := isPrimarySaleTemplateAttribute(ordered[i])
		right := isPrimarySaleTemplateAttribute(ordered[j])
		if left != right {
			return left
		}
		return false
	})
	return ordered
}

func isPrimarySaleTemplateAttribute(attr sheinattribute.AttributeInfo) bool {
	// SHEIN marks the authoritative primary sale attribute with attribute_label=1.
	// This must outrank template array order, SKC scope, required flags, value fit, and variant distinctness.
	return attr.AttributeLabel == 1
}

func firstSaleAttributeTemplate(attributes []sheinattribute.AttributeInfo) *sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return nil
	}
	return &attributes[0]
}

func hasMarkedPrimarySaleTemplate(attributes []sheinattribute.AttributeInfo) bool {
	first := firstSaleAttributeTemplate(attributes)
	return first != nil && isPrimarySaleTemplateAttribute(*first)
}

func buildPrimarySaleTemplateMismatchNote(primary *saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) string {
	first := firstSaleAttributeTemplate(attributes)
	if first == nil {
		return "SHEIN 销售属性首个模板未能安全映射，已阻止使用非首位模板属性作为主销售属性"
	}
	firstName := firstNonEmpty(first.AttributeNameEn, first.AttributeName, strconv.Itoa(first.AttributeID))
	if primary == nil {
		return fmt.Sprintf("SHEIN 销售属性首个模板 %q(ID:%d) 未能安全映射，需确认主销售属性", firstName, first.AttributeID)
	}
	primaryName := firstNonEmpty(primary.TemplateName, primary.Match.Name, strconv.Itoa(primary.AttributeID))
	return fmt.Sprintf(
		"SHEIN 销售属性需按模板顺序，首个模板 %q(ID:%d) 未能安全映射，已阻止使用 %q(ID:%d) 作为主销售属性",
		firstName,
		first.AttributeID,
		primaryName,
		primary.AttributeID,
	)
}
