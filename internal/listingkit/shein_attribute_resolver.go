package listingkit

import (
	"strings"

	"task-processor/internal/productenrich"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sheinAttributeResolver struct {
	api SheinAttributeAPI
}

func NewSheinAttributeResolver(api SheinAttributeAPI) SheinAttributeResolver {
	return &sheinAttributeResolver{api: api}
}

func (r *sheinAttributeResolver) Resolve(req *GenerateRequest, canonical *productenrich.CanonicalProduct, pkg *SheinPackage) *SheinAttributeResolution {
	resolution := &SheinAttributeResolution{
		Status:     "unresolved",
		Source:     "fallback",
		CategoryID: sheinCategoryID(pkg),
	}
	if resolution.CategoryID == 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN category_id，无法加载属性模板")
		return resolution
	}
	if r.api == nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN AttributeAPI，当前无法加载在线属性模板")
		return resolution
	}

	templates, err := r.api.GetAttributeTemplates(resolution.CategoryID)
	if err != nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 属性模板加载失败: "+err.Error())
		return resolution
	}
	if templates == nil || len(templates.Data) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 属性模板为空")
		return resolution
	}

	resolution.Source = "attribute_templates"
	resolution.TemplateCount = len(templates.Data)
	resolution.ResolvedAttributes = matchSheinAttributes(templates, pkg)
	for _, item := range resolution.ResolvedAttributes {
		if item.AttributeID > 0 {
			resolution.ResolvedCount++
		} else {
			resolution.UnresolvedCount++
		}
	}
	if resolution.ResolvedCount > 0 && resolution.UnresolvedCount == 0 {
		resolution.Status = "resolved"
	} else if resolution.ResolvedCount > 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "部分 SHEIN 属性已映射到真实 attribute_id，其余属性仍需人工确认")
	} else {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 属性模板已加载，但暂未命中可映射属性")
	}
	return resolution
}

func matchSheinAttributes(templates *sheinattribute.AttributeTemplateInfo, pkg *SheinPackage) []SheinResolvedAttribute {
	if templates == nil || len(templates.Data) == 0 || pkg == nil {
		return nil
	}
	inputs := buildSheinAttributeInputs(pkg)
	if len(inputs) == 0 {
		return nil
	}

	templateIndex := newSheinTemplateIndex(templates.Data[0].AttributeInfos)
	resolved := make([]SheinResolvedAttribute, 0, len(inputs))
	for _, item := range inputs {
		match := templateIndex.Match(item.Name, item.Value)
		if match.AttributeID == 0 {
			resolved = append(resolved, SheinResolvedAttribute{
				Name:                item.Name,
				Value:               item.Value,
				AttributeExtraValue: item.Value,
				MatchedBy:           "unmatched",
			})
			continue
		}
		resolved = append(resolved, match)
	}
	return resolved
}

func buildSheinAttributeInputs(pkg *SheinPackage) []PlatformAttribute {
	if pkg == nil {
		return nil
	}
	if len(pkg.ProductAttributes) > 0 {
		return append([]PlatformAttribute(nil), pkg.ProductAttributes...)
	}
	if len(pkg.Attributes) == 0 {
		return nil
	}
	result := make([]PlatformAttribute, 0, len(pkg.Attributes))
	for name, value := range pkg.Attributes {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result = append(result, PlatformAttribute{Name: name, Value: value})
	}
	return result
}

func sheinCategoryID(pkg *SheinPackage) int {
	if pkg == nil {
		return 0
	}
	if pkg.CategoryID > 0 {
		return pkg.CategoryID
	}
	if pkg.CategoryResolution != nil {
		return pkg.CategoryResolution.CategoryID
	}
	return 0
}
