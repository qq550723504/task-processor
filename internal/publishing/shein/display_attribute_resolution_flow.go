package shein

import (
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type displayAttributeInput struct {
	Source common.Attribute
	Attr   sheinattribute.AttributeInfo
}

func resolveDisplayAttributes(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) ([]ResolvedAttribute, []common.Attribute, []PendingAttributeCandidate, []PendingAttributeCandidate, []string) {
	if len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil, nil, nil, nil
	}
	templateIndex := newDisplayTemplateIndex(attributes)
	resolved := make([]ResolvedAttribute, 0, len(inputs))
	notes := make([]string, 0)
	matchedInputs := make(map[string]struct{}, len(inputs))

	independent := make([]displayAttributeInput, 0, len(inputs))
	dependent := make([]displayAttributeInput, 0, len(inputs))
	for _, item := range inputs {
		attr, reasons := selectDisplayTemplateAttribute(templateIndex.attributes, item, inputs, llm)
		notes = append(notes, reasons...)
		if attr == nil {
			notes = append(notes, fmt.Sprintf("SHEIN 普通属性模板未命中: 源属性 %q 当前未找到类目内对应模板字段", strings.TrimSpace(item.Name)))
			continue
		}
		entry := displayAttributeInput{Source: item, Attr: *attr}
		if attr.CascadeAttributeID > 0 {
			dependent = append(dependent, entry)
			continue
		}
		independent = append(independent, entry)
	}

	resolvedByID := make(map[int]ResolvedAttribute, len(inputs))
	resolveOne := func(entry displayAttributeInput) {
		match, matchNotes := matchTemplateAttributeValue(entry.Attr, entry.Source.Name, entry.Source.Value, inputs, llm)
		if match.AttributeID == 0 {
			notes = append(notes, matchNotes...)
			return
		}
		if _, exists := resolvedByID[match.AttributeID]; exists {
			notes = append(notes, matchNotes...)
			notes = append(notes, fmt.Sprintf(
				"SHEIN 展示属性去重: 属性 %q 已由更高优先级来源命中，跳过重复来源 %q",
				strings.TrimSpace(match.Name),
				strings.TrimSpace(entry.Source.Name),
			))
			return
		}
		resolved = append(resolved, match)
		matchedInputs[normalizeText(entry.Source.Name)] = struct{}{}
		if match.AttributeID > 0 {
			resolvedByID[match.AttributeID] = match
		}
		notes = append(notes, matchNotes...)
	}

	for _, entry := range independent {
		resolveOne(entry)
	}

	for _, entry := range dependent {
		if !dependencyIsActiveWithInputs(entry.Attr, resolvedByID, inputs) {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 联动属性暂未生效: 源属性 %q 当前依赖上游属性 %d，暂不纳入展示属性映射",
				strings.TrimSpace(entry.Source.Name),
				entry.Attr.CascadeAttributeID,
			))
			continue
		}
		resolveOne(entry)
	}

	inferred, inferNotes := inferMissingRequiredDisplayAttributes(templateIndex.attributes, inputs, resolvedByID, llm)
	if len(inferred) > 0 {
		resolved = append(resolved, inferred...)
	}
	notes = append(notes, inferNotes...)

	batchInferred, batchInferNotes := inferMissingRequiredDisplayAttributesBatch(templateIndex.attributes, inputs, resolvedByID, llm)
	if len(batchInferred) > 0 {
		resolved = append(resolved, batchInferred...)
	}
	notes = append(notes, batchInferNotes...)

	repairInferred, repairInferNotes := inferMissingRequiredDisplayAttributesRepair(templateIndex.attributes, inputs, resolvedByID, llm)
	if len(repairInferred) > 0 {
		resolved = append(resolved, repairInferred...)
	}
	notes = append(notes, repairInferNotes...)

	semanticInferred, semanticInferNotes := inferMissingRequiredDisplayAttributesFromCandidateSemantics(templateIndex.attributes, inputs, resolvedByID)
	if len(semanticInferred) > 0 {
		resolved = append(resolved, semanticInferred...)
	}
	notes = append(notes, semanticInferNotes...)

	pending := buildDependencyPendingAttributes(templateIndex.attributes, resolved, inputs)
	pendingCandidates := buildPendingAttributeCandidates(templateIndex.attributes, resolved, inputs)
	recommendedCandidates := buildRecommendedAttributeCandidates(templateIndex.attributes, resolved, inputs)
	for _, attr := range pending {
		notes = append(notes, fmt.Sprintf("SHEIN 必填展示属性缺失: 模板属性 %q 当前未从源商品中提取到值", strings.TrimSpace(attr.Name)))
	}
	return resolved, pending, pendingCandidates, recommendedCandidates, dedupeStrings(notes)
}
