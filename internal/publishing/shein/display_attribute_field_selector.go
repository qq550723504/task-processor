package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type displayAttributeFieldSelection struct {
	AttributeID int      `json:"attribute_id"`
	Reasons     []string `json:"reasons,omitempty"`
}

type displayAttributeFieldBatchSelection struct {
	Selections []displayAttributeFieldBatchChoice `json:"selections,omitempty"`
}

type displayAttributeFieldBatchChoice struct {
	SourceIndex int      `json:"source_index,omitempty"`
	AttributeID int      `json:"attribute_id,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

const maxDisplayAttributeFieldSelectionBatchSources = 4

func selectDisplayTemplateAttribute(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
	contextInputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (*sheinattribute.AttributeInfo, []string) {
	if attr, notes := selectDisplayTemplateAttributeExact(attributes, source); attr != nil {
		return attr, notes
	}
	if llm == nil || len(attributes) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildDisplayAttributeFieldSelectionPrompt(attributes, source, contextInputs))
	if err != nil {
		return nil, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return nil, nil
	}

	var selection displayAttributeFieldSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		return nil, nil
	}
	if selection.AttributeID <= 0 {
		return nil, selection.Reasons
	}
	for _, attr := range attributes {
		if attr.AttributeID != selection.AttributeID {
			continue
		}
		attrCopy := attr
		return &attrCopy, selection.Reasons
	}
	return nil, selection.Reasons
}

func selectDisplayTemplateAttributesBatch(
	attributes []sheinattribute.AttributeInfo,
	sources []common.Attribute,
	contextInputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (map[int]*sheinattribute.AttributeInfo, []string) {
	results := make(map[int]*sheinattribute.AttributeInfo)
	if llm == nil || len(attributes) == 0 || len(sources) == 0 {
		return results, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildDisplayAttributeFieldSelectionBatchPrompt(attributes, sources, contextInputs))
	if err != nil {
		return results, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return results, nil
	}

	var payload displayAttributeFieldBatchSelection
	if err := json.Unmarshal([]byte(response), &payload); err != nil {
		var choices []displayAttributeFieldBatchChoice
		if arrayErr := json.Unmarshal([]byte(response), &choices); arrayErr != nil {
			return results, nil
		}
		payload.Selections = choices
	}
	notes := make([]string, 0, len(payload.Selections))
	usedAttrIDs := make(map[int]struct{}, len(payload.Selections))
	for _, choice := range payload.Selections {
		notes = append(notes, choice.Reasons...)
		if choice.SourceIndex < 0 || choice.SourceIndex >= len(sources) || choice.AttributeID <= 0 {
			continue
		}
		if _, used := usedAttrIDs[choice.AttributeID]; used {
			continue
		}
		for _, attr := range attributes {
			if attr.AttributeID != choice.AttributeID {
				continue
			}
			attrCopy := attr
			results[choice.SourceIndex] = &attrCopy
			usedAttrIDs[choice.AttributeID] = struct{}{}
			break
		}
	}
	return results, dedupeStrings(notes)
}

func buildDisplayAttributeFieldSelectionPrompt(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
	contextInputs []common.Attribute,
) string {
	var candidateBuilder strings.Builder
	for _, attr := range attributes {
		candidateBuilder.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q name_en=%q type=%d required=%t cascade_attribute_id=%d candidate_values=%d\n",
			attr.AttributeID,
			attr.AttributeName,
			attr.AttributeNameEn,
			attr.AttributeType,
			isTemplateRequired(attr),
			attr.CascadeAttributeID,
			len(attr.AttributeValueInfoList),
		))
	}
	contextBlock := ""
	if context := buildDisplayAttributeContextLines(contextInputs, source.Name, source.Value); len(context) > 0 {
		contextBlock = "Additional source context:\n"
		for _, line := range context {
			contextBlock += "- " + line + "\n"
		}
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeFieldSelection, `You map one source product attribute to one SHEIN display attribute field from the current category template.
Choose exactly one attribute_id only when the semantic match is safe.
If none is safe, return attribute_id as 0.
Return JSON only with keys attribute_id and reasons.

Source attribute: {{.SourceAttribute}}
Source value: {{.SourceValue}}
{{.AdditionalContextBlock}}Candidate SHEIN display attributes:
{{.CandidatesBlock}}`, map[string]any{
		"SourceAttribute":        fmt.Sprintf("%q", strings.TrimSpace(source.Name)),
		"SourceValue":            fmt.Sprintf("%q", strings.TrimSpace(source.Value)),
		"AdditionalContextBlock": contextBlock,
		"CandidatesBlock":        candidateBuilder.String(),
	})
}

func buildDisplayAttributeFieldSelectionBatchPrompt(
	attributes []sheinattribute.AttributeInfo,
	sources []common.Attribute,
	contextInputs []common.Attribute,
) string {
	var sourceBuilder strings.Builder
	for idx, source := range sources {
		sourceBuilder.WriteString(fmt.Sprintf(
			"- source_index=%d name=%q value=%q\n",
			idx,
			strings.TrimSpace(source.Name),
			strings.TrimSpace(source.Value),
		))
	}
	var candidateBuilder strings.Builder
	for _, attr := range attributes {
		candidateBuilder.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q name_en=%q type=%d required=%t cascade_attribute_id=%d candidate_values=%d\n",
			attr.AttributeID,
			attr.AttributeName,
			attr.AttributeNameEn,
			attr.AttributeType,
			isTemplateRequired(attr),
			attr.CascadeAttributeID,
			len(attr.AttributeValueInfoList),
		))
	}
	contextBlock := ""
	if context := buildAllDisplayAttributeContextLines(contextInputs); len(context) > 0 {
		contextBlock = "Additional source context:\n"
		for _, line := range context {
			contextBlock += "- " + line + "\n"
		}
	}
	return renderSheinDisplayAttributePrompt("shein.display_attribute.field_selection_batch", `You map multiple source product attributes to SHEIN display attribute fields from the current category template.
Choose at most one template attribute per source_index and do not assign the same attribute_id to multiple sources.
Only choose an attribute_id when the semantic match is safe. Skip uncertain mappings.
Return JSON only with key selections, where each item is {"source_index":number,"attribute_id":number,"reasons":[string]}.

Source attributes:
{{.SourcesBlock}}
{{.AdditionalContextBlock}}Candidate SHEIN display attributes:
{{.CandidatesBlock}}`, map[string]any{
		"SourcesBlock":           sourceBuilder.String(),
		"AdditionalContextBlock": contextBlock,
		"CandidatesBlock":        candidateBuilder.String(),
	})
}
