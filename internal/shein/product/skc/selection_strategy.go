package skc

import (
	"fmt"
	"strings"

	"task-processor/internal/pkg/types"
	shein "task-processor/internal/shein"
	apiattribute "task-processor/internal/shein/api/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func BuildStrategyFromSelection(ctx *shein.TaskContext, saleSpec *shein.ResultSaleAttribute, attributeTemplates *apiattribute.AttributeTemplateInfo) (sheinattr.AttributeStrategy, shein.ResultSaleAttribute, string, error) {
	if ctx == nil || ctx.SaleAttributeSelection == nil || saleSpec == nil {
		return sheinattr.AttributeStrategy{}, shein.ResultSaleAttribute{}, "legacy", fmt.Errorf("sale attribute selection is unavailable")
	}

	adapted := cloneSaleSpecResult(*saleSpec)
	selection := ctx.SaleAttributeSelection

	primary, err := materializeSelectionAttribute(&adapted, selection.PrimaryAttributeID, selection.PrimarySourceDimension, attributeTemplates)
	if err != nil {
		return sheinattr.AttributeStrategy{}, shein.ResultSaleAttribute{}, "legacy", err
	}

	secondary := sheinattr.ResultAttribute{}
	if selection.SecondaryAttributeID > 0 && strings.TrimSpace(selection.SecondarySourceDimension) != "" {
		secondary, err = materializeSelectionAttribute(&adapted, selection.SecondaryAttributeID, selection.SecondarySourceDimension, attributeTemplates)
		if err != nil {
			return sheinattr.AttributeStrategy{}, shein.ResultSaleAttribute{}, "legacy", err
		}
	}

	strategyType := "selection_primary_only"
	if secondary.AttrID > 0 && len(secondary.AttrValue) > 0 {
		strategyType = "selection_primary_secondary"
	}

	return sheinattr.AttributeStrategy{
		PrimaryAttribute:   primary,
		SecondaryAttribute: secondary,
		StrategyType:       strategyType,
	}, adapted, "selection", nil
}

func materializeSelectionAttribute(saleSpec *shein.ResultSaleAttribute, attributeID int, sourceDimension string, templates *apiattribute.AttributeTemplateInfo) (sheinattr.ResultAttribute, error) {
	if attributeID <= 0 {
		return sheinattr.ResultAttribute{}, fmt.Errorf("selection attribute id is invalid")
	}
	if strings.TrimSpace(sourceDimension) == "" {
		return sheinattr.ResultAttribute{}, fmt.Errorf("selection source dimension is empty")
	}

	targetNames := selectionAttributeNames(attributeID, templates)
	if len(targetNames) == 0 {
		return sheinattr.ResultAttribute{}, fmt.Errorf("template attribute names are unavailable for attribute %d", attributeID)
	}
	values := collectSourceDimensionValues(saleSpec, sourceDimension)
	if len(values) == 0 {
		return sheinattr.ResultAttribute{}, fmt.Errorf("source dimension %q has no values", sourceDimension)
	}
	if existing, ok := findResultAttributeByID(saleSpec, attributeID); ok && len(existing.AttrValue) > 0 && resultAttributeValuesMatch(existing, values) {
		adaptVariantsForSelectionAttribute(saleSpec, targetNames, sourceDimension)
		return cloneResultAttribute(existing), nil
	}

	attrValues := make([]sheinattr.AttributeValue, 0, len(values))
	for idx, value := range values {
		attrValues = append(attrValues, sheinattr.AttributeValue{
			ID:    types.FlexibleID(-(idx + 1)),
			Value: value,
		})
	}

	adaptVariantsForSelectionAttribute(saleSpec, targetNames, sourceDimension)
	return sheinattr.ResultAttribute{
		AttrID:    attributeID,
		AttrValue: attrValues,
	}, nil
}

func adaptVariantsForSelectionAttribute(saleSpec *shein.ResultSaleAttribute, targetNames []string, sourceDimension string) {
	for idx := range saleSpec.Variants {
		variant := &saleSpec.Variants[idx]
		value, ok := lookupVariantAttributeValue(variant.Attributes, sourceDimension)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if variant.Attributes == nil {
			variant.Attributes = map[string]string{}
		}
		for _, name := range targetNames {
			if strings.TrimSpace(name) == "" {
				continue
			}
			variant.Attributes[name] = value
		}
	}
}

func selectionAttributeNames(attributeID int, templates *apiattribute.AttributeTemplateInfo) []string {
	if templates == nil {
		return nil
	}

	var names []string
	for _, data := range templates.Data {
		for _, info := range data.AttributeInfos {
			if info.AttributeID != attributeID {
				continue
			}
			if strings.TrimSpace(info.AttributeName) != "" {
				names = append(names, info.AttributeName)
			}
			if strings.TrimSpace(info.AttributeNameEn) != "" {
				names = append(names, info.AttributeNameEn)
			}
		}
	}
	return uniqueStrings(names)
}

func resultAttributeValuesMatch(attr sheinattr.ResultAttribute, values []string) bool {
	if len(attr.AttrValue) != len(values) {
		return false
	}
	seen := map[string]struct{}{}
	for _, item := range attr.AttrValue {
		seen[strings.ToLower(strings.TrimSpace(item.Value))] = struct{}{}
	}
	for _, value := range values {
		if _, ok := seen[strings.ToLower(strings.TrimSpace(value))]; !ok {
			return false
		}
	}
	return true
}

func collectSourceDimensionValues(saleSpec *shein.ResultSaleAttribute, sourceDimension string) []string {
	if saleSpec == nil {
		return nil
	}

	var values []string
	seen := map[string]struct{}{}
	for _, variant := range saleSpec.Variants {
		value, ok := lookupVariantAttributeValue(variant.Attributes, sourceDimension)
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, strings.TrimSpace(value))
	}
	return values
}

func lookupVariantAttributeValue(attributes map[string]string, sourceDimension string) (string, bool) {
	if len(attributes) == 0 {
		return "", false
	}

	for key, value := range attributes {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(sourceDimension)) {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func findResultAttributeByID(saleSpec *shein.ResultSaleAttribute, attributeID int) (sheinattr.ResultAttribute, bool) {
	if saleSpec == nil {
		return sheinattr.ResultAttribute{}, false
	}
	for _, attr := range saleSpec.SaleAttributes {
		if attr.AttrID == attributeID {
			return attr, true
		}
	}
	return sheinattr.ResultAttribute{}, false
}

func cloneSaleSpecResult(in shein.ResultSaleAttribute) shein.ResultSaleAttribute {
	out := shein.ResultSaleAttribute{
		SaleAttributes: make([]shein.ResultAttribute, 0, len(in.SaleAttributes)),
		Variants:       make([]shein.Variant, 0, len(in.Variants)),
	}
	for _, attr := range in.SaleAttributes {
		out.SaleAttributes = append(out.SaleAttributes, cloneResultAttribute(attr))
	}
	for _, variant := range in.Variants {
		cloned := shein.Variant{
			Attributes:   map[string]string{},
			Length:       variant.Length,
			Width:        variant.Width,
			Height:       variant.Height,
			Weight:       variant.Weight,
			LengthUnit:   variant.LengthUnit,
			ASIN:         variant.ASIN,
			Price:        variant.Price,
			QuantityType: variant.QuantityType,
			UnitType:     variant.UnitType,
			Quantity:     variant.Quantity,
		}
		for key, value := range variant.Attributes {
			cloned.Attributes[key] = value
		}
		out.Variants = append(out.Variants, cloned)
	}
	return out
}

func cloneResultAttribute(in shein.ResultAttribute) shein.ResultAttribute {
	out := shein.ResultAttribute{
		AttrID:    in.AttrID,
		AttrValue: make([]shein.AttributeValue, 0, len(in.AttrValue)),
	}
	for _, value := range in.AttrValue {
		out.AttrValue = append(out.AttrValue, value)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
