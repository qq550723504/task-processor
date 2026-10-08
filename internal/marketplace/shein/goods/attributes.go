package goods

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	model "task-processor/internal/marketplace/shein/model"
)

func (b *officialBuild) attribute(id int64) (model.Attribute, bool) {
	for _, attribute := range b.rules.Attributes.Attributes {
		if attribute.ID == id {
			return attribute, true
		}
	}
	return model.Attribute{}, false
}
func attributeDimension(attribute model.Attribute, dimension int) bool {
	return dimension == 0 || attribute.Dimension != nil && *attribute.Dimension == dimension
}
func (b *officialBuild) validateAttributes(path string, list []model.AttributeValue, firstType, lastType, dimension int, skipID int64) {
	b.validateAttributeValues(path, list, firstType, lastType, dimension, skipID, true)
}
func (b *officialBuild) validateAttributeValues(path string, list []model.AttributeValue, firstType, lastType, dimension int, skipID int64, required bool) {
	if len(list) > 1000 {
		b.issue(path, "invalid", "属性数量超过限制")
		return
	}
	byID := map[int64][]model.AttributeValue{}
	for _, value := range list {
		byID[value.AttributeID] = append(byID[value.AttributeID], value)
	}
	for _, attribute := range b.rules.Attributes.Attributes {
		if attribute.Type == nil || *attribute.Type < firstType || *attribute.Type > lastType || !attributeDimension(attribute, dimension) || attribute.ID == skipID {
			continue
		}
		if required && attribute.Status != nil && *attribute.Status == 3 && len(byID[attribute.ID]) == 0 {
			b.issue(fmt.Sprintf("%s.%d", path, attribute.ID), "missing", "填写必填属性："+attribute.Name)
		}
	}
	for id, values := range byID {
		attribute, ok := b.attribute(id)
		field := fmt.Sprintf("%s.%d", path, id)
		if !ok || attribute.Type == nil || *attribute.Type < firstType || *attribute.Type > lastType || !attributeDimension(attribute, dimension) || attribute.Status == nil || *attribute.Status == 1 || attribute.Show == nil || *attribute.Show != 1 || attribute.Mode == nil {
			b.issue(field, "invalid", "属性不属于当前层级的可用属性")
			continue
		}
		mode := *attribute.Mode
		if (mode == 2 || mode == 3 || mode == 0) && len(values) != 1 || attribute.MaximumSelections != nil && *attribute.MaximumSelections > 0 && len(values) > *attribute.MaximumSelections {
			b.issue(field, "invalid", "所选属性值数量不符合当前规则")
		}
		seen := map[string]bool{}
		compositionSum := 0.0
		for _, value := range values {
			if value.CustomValue != "" || value.Language != "" {
				b.issue(field, "unsupported", "请使用当前店铺已存在的属性值；创建自定义属性值尚未开放")
			}
			if mode == 0 {
				if value.AttributeValueID != nil || !positiveInteger(value.ExtraValue) {
					b.issue(field, "invalid", "此属性须填写正整数")
				}
			} else {
				found := false
				for _, option := range attribute.Options {
					if value.AttributeValueID != nil && option.ID == *value.AttributeValueID && option.Show != nil && *option.Show == 1 {
						found = true
					}
				}
				if !found {
					b.issue(field, "invalid", "选择当前店铺可用的属性值")
				}
				if mode != 4 && value.ExtraValue != "" {
					b.issue(field, "invalid", "此属性不接受手动输入值")
				}
			}
			if mode == 4 {
				if !officialText(value.ExtraValue, 1, 500) {
					b.issue(field, "missing", "填写此属性的手动输入值")
				}
				if *attribute.Type == 3 {
					number, err := strconv.ParseFloat(value.ExtraValue, 64)
					if err != nil || number <= 0 || number > 100 {
						b.issue(field, "invalid", "成分比例须为 0 至 100 之间的正数")
					}
					compositionSum += number
				}
			}
			identity, _ := json.Marshal(value)
			if seen[string(identity)] {
				b.issue(field, "invalid", "属性值重复")
			}
			seen[string(identity)] = true
			for _, rule := range attribute.Rules {
				if value.ExtraValue == "" {
					continue
				}
				if rule.Type == nil || rule.Operator == nil {
					b.issue(field, "rule_unavailable", "此属性输入规则不完整")
					continue
				}
				switch *rule.Type {
				case 1:
					if !positiveInteger(value.ExtraValue) {
						b.issue(field, "invalid", "此属性仅支持正整数")
					}
				case 3:
					if !officialDecimal.MatchString(value.ExtraValue) {
						b.issue(field, "invalid", "此属性须填写规范的小数")
					}
				case 4:
					number, err := strconv.ParseInt(value.ExtraValue, 10, 64)
					if err != nil || number < 0 {
						b.issue(field, "invalid", "此属性须填写自然数")
					}
				default:
					b.issue(field, "unsupported_rule", "此属性的输入规则暂未开放")
				}
				if *rule.Operator != 0 {
					number, nerr := strconv.ParseFloat(value.ExtraValue, 64)
					limit, lerr := strconv.ParseFloat(rule.Value, 64)
					if nerr != nil || lerr != nil || *rule.Operator == 1 && number <= limit || *rule.Operator == 2 && number >= limit || *rule.Operator != 1 && *rule.Operator != 2 {
						b.issue(field, "invalid", "此属性输入值不符合范围规则")
					}
				}
			}
		}
		if mode == 4 && *attribute.Type == 3 && (compositionSum < 99.999999 || compositionSum > 100.000001) {
			b.issue(field, "invalid", "同一成分属性的比例总和须为 100")
		}
	}
}
func positiveInteger(value string) bool {
	number, err := strconv.ParseInt(value, 10, 64)
	return err == nil && number > 0 && strconv.FormatInt(number, 10) == value
}
func (b *officialBuild) validateMainAttribute(path string, value model.AttributeValue, skcCount int) {
	attribute, ok := b.attribute(value.AttributeID)
	if !ok || attribute.Type == nil || *attribute.Type != 1 || attribute.MainLabel == nil || *attribute.MainLabel != 1 {
		b.issue(path, "missing", "选择当前店铺的 SKC 主销售属性")
		return
	}
	b.validateAttributeValues(path, []model.AttributeValue{value}, 1, 1, 0, 0, false)
	if b.rules.Attributes.MainAttributeStatus != nil && *b.rules.Attributes.MainAttributeStatus == 1 && (skcCount != 1 || !strings.EqualFold(attribute.Name, "default") && attribute.Name != "默认") {
		b.issue(path, "invalid", "当前类目仅允许一个 SKC，且主销售属性须选择店铺提供的默认属性")
	}
}
func (b *officialBuild) validateSizes(list []model.SizeAttribute) {
	byID := map[int64]bool{}
	for _, value := range list {
		attribute, ok := b.attribute(value.AttributeID)
		field := fmt.Sprintf("size_attribute_list.%d", value.AttributeID)
		if !ok || attribute.Type == nil || *attribute.Type != 2 || attribute.Status == nil || *attribute.Status == 1 || !positiveInteger(value.ExtraValue) {
			b.issue(field, "invalid", "填写可用尺码属性的真实正整数")
		}
		if (value.RelatedSaleAttributeID == nil) != (value.RelatedSaleAttributeValueID == nil) {
			b.issue(field, "invalid", "尺码表关联规格不完整")
		}
		if value.RelatedSaleAttributeID != nil {
			matched := false
			for _, skc := range b.result.Product.SKCs {
				for _, sku := range skc.SKUs {
					for _, sale := range sku.SaleAttributes {
						if sale.AttributeID == *value.RelatedSaleAttributeID && sale.AttributeValueID != nil && *sale.AttributeValueID == *value.RelatedSaleAttributeValueID {
							matched = true
						}
					}
				}
			}
			if !matched {
				b.issue(field, "invalid", "尺码表须关联本商品实际 SKU 规格")
			}
		}
		byID[value.AttributeID] = true
	}
	for _, attribute := range b.rules.Attributes.Attributes {
		if attribute.Type != nil && *attribute.Type == 2 && attribute.Status != nil && *attribute.Status == 3 && !byID[attribute.ID] {
			b.issue(fmt.Sprintf("size_attribute_list.%d", attribute.ID), "missing", "填写必填尺码属性："+attribute.Name)
		}
	}
}
func saleSignature(values []model.AttributeValue, skipID int64) string {
	parts := []string{}
	for _, value := range values {
		if value.AttributeID != skipID {
			raw, _ := json.Marshal(value)
			parts = append(parts, string(raw))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}
func (b *officialBuild) validateVariantMatrix() {
	var first []string
	for i, skc := range b.result.Product.SKCs {
		signatures := []string{}
		seen := map[string]bool{}
		for _, sku := range skc.SKUs {
			signature := saleSignature(sku.SaleAttributes, skc.SaleAttribute.AttributeID)
			if seen[signature] {
				b.issue(fmt.Sprintf("skc_list.%d.sku_list", i), "invalid", "同一 SKC 的 SKU 销售规格不得重复")
			}
			seen[signature] = true
			signatures = append(signatures, signature)
		}
		sort.Strings(signatures)
		if i == 0 {
			first = signatures
		} else if strings.Join(first, "\x00") != strings.Join(signatures, "\x00") {
			b.issue(fmt.Sprintf("skc_list.%d.sku_list", i), "invalid", "各 SKC 下的 SKU 规格范围须一致")
		}
	}
}
func (b *officialBuild) validateLinkedRules() {
	productGroups := 0
	for _, group := range b.rules.Linked {
		if group.GroupID == "product" {
			productGroups++
		}
	}
	if productGroups != 1 {
		b.issue("linked_attributes", "rule_unavailable", "当前属性组合的关联必填规则未查询")
		return
	}
	for _, group := range b.rules.Linked {
		for _, rule := range group.Attributes {
			attribute, ok := b.attribute(rule.AttributeID)
			if !ok || attribute.Type == nil {
				b.issue("linked_attributes", "rule_unavailable", "关联必填属性不在当前类目规范中")
				continue
			}
			allowed := map[int64]bool{}
			for _, id := range append(append([]int64(nil), rule.Values...), rule.PrefilledValues...) {
				allowed[id] = true
			}
			if *attribute.Type == 2 {
				found := false
				for _, value := range b.result.Product.SizeAttributes {
					found = found || value.AttributeID == rule.AttributeID
				}
				if !found || len(allowed) > 0 {
					b.issue(fmt.Sprintf("size_attribute_list.%d", rule.AttributeID), "missing", "补齐关联必填尺码属性及其范围")
				}
				continue
			}
			check := func(path string, values []model.AttributeValue) {
				count := 0
				valid := true
				for _, value := range values {
					if value.AttributeID == rule.AttributeID {
						count++
						valid = valid && (len(allowed) == 0 || value.AttributeValueID != nil && allowed[*value.AttributeValueID])
					}
				}
				if count == 0 || !valid {
					b.issue(fmt.Sprintf("%s.%d", path, rule.AttributeID), "missing", "填写关联必填属性，并选择规则允许的值")
				}
			}
			if attribute.Dimension != nil && *attribute.Dimension == 1 {
				check("product_attribute_list", b.result.Product.Attributes)
			} else if attribute.Dimension != nil && *attribute.Dimension == 3 {
				for i, skc := range b.result.Product.SKCs {
					for j, sku := range skc.SKUs {
						check(fmt.Sprintf("skc_list.%d.sku_list.%d.sku_scope_attribute_list", i, j), sku.Attributes)
					}
				}
			} else {
				b.issue("linked_attributes", "rule_unavailable", "关联属性所属层级不可用")
			}
		}
	}
}
