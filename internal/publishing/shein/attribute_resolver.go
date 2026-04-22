package shein

import (
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type attributeResolver struct {
	api AttributeAPI
	llm openaiclient.ChatCompleter
}

func NewAttributeResolver(api AttributeAPI, llm openaiclient.ChatCompleter) AttributeResolver {
	return &attributeResolver{api: api, llm: llm}
}

func (r *attributeResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *AttributeResolution {
	resolution := &AttributeResolution{Status: "unresolved", Source: "fallback", CategoryID: categoryID(pkg)}
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
	resolution.ResolvedAttributes, resolution.ReviewNotes = matchAttributes(templates, pkg, r.llm)
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

func matchAttributes(templates *sheinattribute.AttributeTemplateInfo, pkg *Package, llm openaiclient.ChatCompleter) ([]ResolvedAttribute, []string) {
	if templates == nil || len(templates.Data) == 0 || pkg == nil {
		return nil, nil
	}
	inputs := buildAttributeInputs(pkg)
	if len(inputs) == 0 {
		return nil, nil
	}
	templateIndex := newTemplateIndex(templates.Data[0].AttributeInfos)
	resolved := make([]ResolvedAttribute, 0, len(inputs))
	notes := make([]string, 0)
	for _, item := range inputs {
		attr := templateIndex.FindAttribute(item.Name)
		if attr == nil || isSaleScopeAttribute(*attr) {
			continue
		}
		match, matchNotes := matchTemplateAttributeValue(*attr, item.Name, item.Value, llm)
		if match.AttributeID == 0 {
			continue
		}
		resolved = append(resolved, match)
		notes = append(notes, matchNotes...)
	}
	return resolved, dedupeStrings(notes)
}

func buildAttributeInputs(pkg *Package) []common.Attribute {
	if pkg == nil {
		return nil
	}
	if len(pkg.ProductAttributes) > 0 {
		return append([]common.Attribute(nil), pkg.ProductAttributes...)
	}
	if len(pkg.Attributes) == 0 {
		return nil
	}
	result := make([]common.Attribute, 0, len(pkg.Attributes))
	for name, value := range pkg.Attributes {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result = append(result, common.Attribute{Name: name, Value: value})
	}
	return result
}

func isSaleScopeAttribute(attr sheinattribute.AttributeInfo) bool {
	return attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope)
}

func categoryID(pkg *Package) int {
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
