package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func repairSheinFreshnessSaleAttributes(
	current *SheinPackage,
	attrByID map[int]sheinattribute.AttributeInfo,
	api sheinpub.AttributeAPI,
) bool {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil || current.SaleAttributeResolution == nil || len(attrByID) == 0 || api == nil || current.CategoryID <= 0 {
		return false
	}

	spuName := strings.TrimSpace(current.SpuName)
	if spuName == "" {
		spuName = strings.TrimSpace(current.ProductNameEn)
	}

	changed := false
	relations := make([]sheinattribute.CustomAttributeRelation, 0)
	for index, item := range current.SaleAttributeResolution.SKCAttributes {
		repaired, itemRelations, ok := tryRepairFreshnessSaleAttribute(item, attrByID, current.CategoryID, spuName, api)
		if !ok {
			continue
		}
		current.SaleAttributeResolution.SKCAttributes[index] = repaired
		relations = append(relations, itemRelations...)
		changed = true
	}
	for index, item := range current.SaleAttributeResolution.SKUAttributes {
		repaired, itemRelations, ok := tryRepairFreshnessSaleAttribute(item, attrByID, current.CategoryID, spuName, api)
		if !ok {
			continue
		}
		current.SaleAttributeResolution.SKUAttributes[index] = repaired
		relations = append(relations, itemRelations...)
		changed = true
	}
	if !changed {
		return false
	}
	if len(relations) > 0 {
		current.SaleAttributeResolution.CustomAttributeRelation = dedupeCustomAttributeRelations(append(
			append([]sheinattribute.CustomAttributeRelation(nil), current.SaleAttributeResolution.CustomAttributeRelation...),
			relations...,
		))
	}
	sheinpub.ApplySaleAttributeResolution(current, current.SaleAttributeResolution)
	return true
}

func tryRepairFreshnessSaleAttribute(
	item sheinpub.ResolvedSaleAttribute,
	attrByID map[int]sheinattribute.AttributeInfo,
	categoryID int,
	spuName string,
	api sheinpub.AttributeAPI,
) (sheinpub.ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, bool) {
	if item.AttributeID <= 0 || strings.TrimSpace(item.Value) == "" {
		return sheinpub.ResolvedSaleAttribute{}, nil, false
	}
	attr, ok := attrByID[item.AttributeID]
	if !ok {
		return sheinpub.ResolvedSaleAttribute{}, nil, false
	}
	repaired, relations, _, matched := sheinpub.ResolveSingleSaleAttributeValue(
		attr,
		firstNonEmpty(strings.TrimSpace(item.Name), firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)),
		item.Value,
		strings.TrimSpace(item.Scope),
		api,
		categoryID,
		spuName,
	)
	if !matched || repaired.AttributeValueID == nil || *repaired.AttributeValueID <= 0 {
		return sheinpub.ResolvedSaleAttribute{}, nil, false
	}
	return repaired, relations, true
}
