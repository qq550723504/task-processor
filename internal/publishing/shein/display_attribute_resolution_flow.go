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
	evidence *DisplayAttributeEvidencePool,
	llm openaiclient.ChatCompleter,
) ([]ResolvedAttribute, []common.Attribute, []PendingAttributeCandidate, []PendingAttributeCandidate, []string) {
	if len(attributes) == 0 || evidence == nil {
		return nil, nil, nil, nil, nil
	}
	inputs := evidence.AttributeInputs()
	resolutionInputs := evidence.ResolutionInputs()
	if len(inputs) == 0 {
		return nil, nil, nil, nil, nil
	}
	templateIndex := newDisplayTemplateIndex(attributes)
	resolved := make([]ResolvedAttribute, 0, len(templateIndex.attributes))
	notes := make([]string, 0)
	independent := make([]displayAttributeInput, 0, len(templateIndex.attributes))
	dependent := make([]displayAttributeInput, 0, len(templateIndex.attributes))
	selectedInputs, selectionNotes := assignDisplayAttributeResolutionInputs(templateIndex.attributes, resolutionInputs, inputs, llm)
	notes = append(notes, selectionNotes...)

	resolvedByID := make(map[int]ResolvedAttribute, len(inputs))
	resolveOne := func(entry displayAttributeInput) {
		match, matchNotes, unresolved, ok := matchTemplateAttributeValueDeterministic(entry.Attr, entry.Source.Name, entry.Source.Value)
		if ok {
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
			if match.AttributeID > 0 {
				resolvedByID[match.AttributeID] = match
			}
			notes = append(notes, matchNotes...)
			return
		}
		_ = unresolved
	}

	for _, attr := range templateIndex.attributes {
		if _, exists := resolvedByID[attr.AttributeID]; exists {
			continue
		}
		entry, ok := selectedInputs[attr.AttributeID]
		if !ok {
			continue
		}
		if attr.CascadeAttributeID > 0 && !dependencyIsActiveWithInputs(attr, resolvedByID, inputs) {
			dependent = append(dependent, entry)
			continue
		}
		independent = append(independent, entry)
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

	templateBatchResolved, templateBatchNotes := inferDisplayAttributesTemplateBatch(templateIndex.attributes, inputs, resolvedByID, llm)
	if len(templateBatchResolved) > 0 {
		resolved = append(resolved, templateBatchResolved...)
	}
	notes = append(notes, templateBatchNotes...)

	pending := buildDependencyPendingAttributes(templateIndex.attributes, resolved, inputs)
	pendingCandidates := buildPendingAttributeCandidates(templateIndex.attributes, resolved, inputs)
	recommendedCandidates := buildRecommendedAttributeCandidates(templateIndex.attributes, resolved, inputs)
	for _, attr := range pending {
		notes = append(notes, fmt.Sprintf("SHEIN 必填展示属性缺失: 模板属性 %q 当前未从源商品中提取到值", strings.TrimSpace(attr.Name)))
	}
	return resolved, pending, pendingCandidates, recommendedCandidates, dedupeStrings(notes)
}

func assignDisplayAttributeResolutionInputs(
	attributes []sheinattribute.AttributeInfo,
	resolutionInputs []common.Attribute,
	contextInputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (map[int]displayAttributeInput, []string) {
	assignments := make(map[int]displayAttributeInput, len(attributes))
	if len(attributes) == 0 || len(resolutionInputs) == 0 {
		return assignments, nil
	}
	notes := make([]string, 0)
	for _, input := range resolutionInputs {
		candidates := remainingDisplayAttributeCandidates(attributes, assignments)
		if len(candidates) == 0 {
			break
		}
		if attr := selectDisplayTemplateAttributeByExactName(candidates, input); attr != nil {
			assignments[attr.AttributeID] = displayAttributeInput{Source: input, Attr: *attr}
			notes = append(notes, fmt.Sprintf(
				"SHEIN 普通属性字段精确匹配: 模板字段 %q 命中源属性 %q",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				strings.TrimSpace(input.Name),
			))
			continue
		}
	}
	_ = contextInputs
	_ = llm
	return assignments, dedupeStrings(notes)
}

func remainingDisplayAttributeCandidates(
	attributes []sheinattribute.AttributeInfo,
	assignments map[int]displayAttributeInput,
) []sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return nil
	}
	candidates := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if _, exists := assignments[attr.AttributeID]; exists {
			continue
		}
		candidates = append(candidates, attr)
	}
	return candidates
}

func selectDisplayTemplateAttributeByExactName(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
) *sheinattribute.AttributeInfo {
	sourceName := normalizeText(source.Name)
	if sourceName == "" {
		return nil
	}
	for _, attr := range attributes {
		if !matchesTemplateAttributeNameExactly(attr, sourceName) {
			continue
		}
		attrCopy := attr
		return &attrCopy
	}
	return nil
}
