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

func selectDisplayTemplateAttribute(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
	contextInputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (*sheinattribute.AttributeInfo, []string) {
	if llm == nil || len(attributes) == 0 {
		if attr, notes := selectDisplayTemplateAttributeExact(attributes, source); attr != nil {
			return attr, notes
		}
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildDisplayAttributeFieldSelectionPrompt(attributes, source, contextInputs))
	if err != nil {
		if attr, notes := selectDisplayTemplateAttributeExact(attributes, source); attr != nil {
			return attr, notes
		}
		return nil, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		if attr, notes := selectDisplayTemplateAttributeExact(attributes, source); attr != nil {
			return attr, notes
		}
		return nil, nil
	}

	var selection displayAttributeFieldSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		if attr, notes := selectDisplayTemplateAttributeExact(attributes, source); attr != nil {
			return attr, notes
		}
		return nil, nil
	}
	if selection.AttributeID <= 0 {
		if attr, notes := selectDisplayTemplateAttributeExact(attributes, source); attr != nil {
			return attr, append(selection.Reasons, notes...)
		}
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
