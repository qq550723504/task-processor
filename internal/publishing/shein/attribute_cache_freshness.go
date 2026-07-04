package shein

import (
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func (r *attributeResolver) CachedAttributeResolutionIsFresh(_ *BuildRequest, _ *canonical.Product, pkg *Package, resolution *AttributeResolution) (bool, string) {
	if r == nil || r.api == nil || resolution == nil {
		return true, ""
	}
	categoryID := categoryID(pkg)
	if categoryID <= 0 {
		categoryID = resolution.CategoryID
	}
	if categoryID <= 0 {
		return true, ""
	}
	templates, err := r.api.GetAttributeTemplates(categoryID)
	if err != nil || templates == nil || len(templates.Data) == 0 {
		return true, ""
	}
	return attributeResolutionMatchesTemplates(resolution, templates)
}

func (r *runtimeAttributeResolver) CachedAttributeResolutionIsFresh(req *BuildRequest, canonical *canonical.Product, pkg *Package, resolution *AttributeResolution) (bool, string) {
	if r == nil {
		return true, ""
	}
	if req == nil {
		if validator, ok := r.fallback.(attributeResolutionCacheValidator); ok {
			return validator.CachedAttributeResolutionIsFresh(req, canonical, pkg, resolution)
		}
		return true, ""
	}
	api, _ := r.buildAPI(req.Context, req.SheinStoreID)
	return (&attributeResolver{api: api}).CachedAttributeResolutionIsFresh(req, canonical, pkg, resolution)
}

func attributeResolutionMatchesTemplates(resolution *AttributeResolution, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	if resolution == nil || templates == nil || len(templates.Data) == 0 {
		return true, ""
	}
	attributeIndex := make(map[int]sheinattribute.AttributeInfo)
	for _, attr := range newDisplayTemplateIndex(templates.Data[0].AttributeInfos).attributes {
		if attr.AttributeID > 0 {
			attributeIndex[attr.AttributeID] = attr
		}
	}
	for _, item := range resolution.ResolvedAttributes {
		if item.AttributeID <= 0 {
			continue
		}
		if !resolvedAttributeMatchesTemplate(item, attributeIndex) {
			return false, fmt.Sprintf("cached attribute %q (attribute_id=%d) is not valid in current template", strings.TrimSpace(item.Name), item.AttributeID)
		}
	}
	return true, ""
}

func resolvedAttributeMatchesTemplate(item ResolvedAttribute, attributeIndex map[int]sheinattribute.AttributeInfo) bool {
	attr, ok := attributeIndex[item.AttributeID]
	if !ok {
		return false
	}
	if item.AttributeValueID != nil && *item.AttributeValueID > 0 {
		for _, option := range attr.AttributeValueInfoList {
			if option.AttributeValueID == *item.AttributeValueID {
				return true
			}
		}
		return false
	}
	if strings.TrimSpace(item.AttributeExtraValue) != "" {
		return true
	}
	return len(attr.AttributeValueInfoList) == 0
}
