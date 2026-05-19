package shein

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
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

func (r *attributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *AttributeResolution {
	resolution := &AttributeResolution{Status: "unresolved", Source: "fallback", CategoryID: categoryID(pkg)}
	log := sheinLogger("shein/attribute")
	if resolution.CategoryID == 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN category_id，无法加载属性模板")
		return resolution
	}
	if r.api == nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN AttributeAPI，当前无法加载在线属性模板")
		return resolution
	}
	log.WithField("category_id", resolution.CategoryID).Info("loading SHEIN display attribute templates")
	templates, err := r.api.GetAttributeTemplates(resolution.CategoryID)
	if err != nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 属性模板加载失败: "+err.Error())
		log.WithError(err).WithField("category_id", resolution.CategoryID).Warn("failed to load SHEIN display attribute templates")
		return resolution
	}
	if templates == nil || len(templates.Data) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 属性模板为空")
		log.WithField("category_id", resolution.CategoryID).Warn("SHEIN display attribute templates are empty")
		return resolution
	}
	log.WithFields(logrus.Fields{
		"category_id":     resolution.CategoryID,
		"template_groups": len(templates.Data),
		"attribute_count": len(templates.Data[0].AttributeInfos),
	}).Info("loaded SHEIN display attribute templates")
	resolution.Source = "attribute_templates"
	resolution.TemplateCount = len(templates.Data)
	resolution.ResolvedAttributes, resolution.PendingAttributes, resolution.PendingAttributeCandidates, resolution.RecommendedAttributeCandidates, resolution.ReviewNotes = matchAttributes(resolveBuildRequestContext(req), templates, pkg, r.llm)
	for _, item := range resolution.ResolvedAttributes {
		if item.AttributeID > 0 {
			resolution.ResolvedCount++
		} else {
			resolution.UnresolvedCount++
		}
	}
	resolution.UnresolvedCount += len(resolution.PendingAttributes)
	if resolution.ResolvedCount > 0 && resolution.UnresolvedCount == 0 {
		resolution.Status = "resolved"
	} else if resolution.ResolvedCount > 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "部分 SHEIN 属性已映射到真实 attribute_id，其余属性仍需人工确认")
	} else {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 属性模板已加载，但暂未命中可映射属性")
	}
	unresolvedNames := make([]string, 0, len(resolution.PendingAttributes))
	for _, item := range resolution.PendingAttributes {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		unresolvedNames = append(unresolvedNames, strings.TrimSpace(item.Name))
	}
	log.WithFields(logrus.Fields{
		"category_id":      resolution.CategoryID,
		"resolved_count":   resolution.ResolvedCount,
		"unresolved_count": resolution.UnresolvedCount,
		"status":           resolution.Status,
		"unresolved_attrs": unresolvedNames,
	}).Info("resolved SHEIN display attributes")
	return resolution
}

func matchAttributes(ctx context.Context, templates *sheinattribute.AttributeTemplateInfo, pkg *Package, llm openaiclient.ChatCompleter) ([]ResolvedAttribute, []common.Attribute, []PendingAttributeCandidate, []PendingAttributeCandidate, []string) {
	if templates == nil || len(templates.Data) == 0 || pkg == nil {
		return nil, nil, nil, nil, nil
	}
	evidence := buildDisplayAttributeEvidencePool(pkg)
	if evidence == nil {
		return nil, nil, nil, nil, nil
	}
	return resolveDisplayAttributes(ctx, templates.Data[0].AttributeInfos, evidence, llm)
}

func buildAttributeInputs(pkg *Package) []common.Attribute {
	evidence := buildDisplayAttributeEvidencePool(pkg)
	if evidence == nil {
		return nil
	}
	return evidence.AttributeInputs()
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

func resolveBuildRequestContext(req *BuildRequest) context.Context {
	if req != nil && req.Context != nil {
		return req.Context
	}
	return context.Background()
}
