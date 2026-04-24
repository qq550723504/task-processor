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
) ([]ResolvedAttribute, []common.Attribute, []string) {
	if len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil, nil
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
		if !dependencyIsActive(entry.Attr, resolvedByID) {
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

	pending := buildDependencyPendingAttributes(templateIndex.attributes, resolved)
	for _, attr := range pending {
		notes = append(notes, fmt.Sprintf("SHEIN 必填展示属性缺失: 模板属性 %q 当前未从源商品中提取到值", strings.TrimSpace(attr.Name)))
	}
	return resolved, pending, dedupeStrings(notes)
}
