package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
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
		if attr, notes := selectDisplayTemplateAttributeStatic(attributes, source); attr != nil {
			return attr, notes
		}
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

func buildDisplayAttributeFieldSelectionPrompt(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
	contextInputs []common.Attribute,
) string {
	var builder strings.Builder
	builder.WriteString("You map one source product attribute to one SHEIN display attribute field from the current category template.\n")
	builder.WriteString("Choose exactly one attribute_id only when the semantic match is safe.\n")
	builder.WriteString("If none is safe, return attribute_id as 0.\n")
	builder.WriteString("Return JSON only with keys attribute_id and reasons.\n\n")
	builder.WriteString(fmt.Sprintf("Source attribute: %q\n", strings.TrimSpace(source.Name)))
	builder.WriteString(fmt.Sprintf("Source value: %q\n", strings.TrimSpace(source.Value)))
	if context := buildDisplayAttributeContextLines(contextInputs, source.Name, source.Value); len(context) > 0 {
		builder.WriteString("Additional source context:\n")
		for _, line := range context {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("Candidate SHEIN display attributes:\n")
	for _, attr := range attributes {
		builder.WriteString(fmt.Sprintf(
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
	return builder.String()
}
