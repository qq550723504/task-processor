package attribute

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
)

type llmPlatformValueFallbackResolver struct {
	client openaiclient.ChatCompleter
}

func NewLLMPlatformValueFallbackResolver(client openaiclient.ChatCompleter) platformValueFallbackResolver {
	if client == nil {
		return nil
	}
	return &llmPlatformValueFallbackResolver{client: client}
}

func (r *llmPlatformValueFallbackResolver) ResolvePlatformValue(ctx context.Context, req *PlatformValueFallbackRequest) (*PlatformValueFallbackResult, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("llm fallback client is not initialized")
	}
	if req == nil {
		return nil, fmt.Errorf("fallback request is nil")
	}

	systemPrompt := defaultPlatformValueFallbackSystemPrompt()
	if prompt.GlobalRegistry != nil {
		systemPrompt = prompt.GlobalRegistry.Get(prompt.KSheinAttributeValueFallbackSystem, defaultPlatformValueFallbackSystemPrompt())
	}
	userPrompt, err := renderPlatformValueFallbackUserPrompt(req)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.CreateChatCompletion(ctx, &openaiclient.ChatCompletionRequest{
		Messages: []openaiclient.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: "json_object",
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm fallback returned empty response")
	}

	var result PlatformValueFallbackResult
	content := jsonx.CleanLLMResponse(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("decode llm fallback response: %w", err)
	}
	result.ResolvedValue = strings.TrimSpace(result.ResolvedValue)
	if result.ResolvedValue == "" {
		return nil, fmt.Errorf("llm fallback returned empty resolved_value")
	}
	return &result, nil
}

func defaultPlatformValueFallbackSystemPrompt() string {
	return "You map a source product variant value to one of the existing platform attribute values. Reply with strict JSON only: {\"resolved_value\":\"...\",\"confidence\":0.0,\"reason\":\"...\"}. Choose only from platform values provided by the user. If unsure, return the closest existing platform value."
}

func renderPlatformValueFallbackUserPrompt(req *PlatformValueFallbackRequest) (string, error) {
	vars := map[string]any{
		"AttrID":         req.AttrID,
		"Domain":         req.Domain,
		"ProductTitle":   req.ProductTitle,
		"RawValue":       req.RawValue,
		"PlatformValues": formatPlatformValuesForPrompt(req.PlatformValues),
		"SizeChart":      defaultString(strings.TrimSpace(req.SizeChart), "(empty)"),
	}
	if prompt.GlobalRegistry != nil {
		rendered, err := prompt.GlobalRegistry.Render(prompt.KSheinAttributeValueFallbackUser, vars, defaultPlatformValueFallbackUserPrompt())
		if err == nil {
			return rendered, nil
		}
	}
	rendered := defaultPlatformValueFallbackUserPrompt()
	for key, value := range vars {
		rendered = strings.ReplaceAll(rendered, "{{."+key+"}}", fmt.Sprint(value))
	}
	return rendered, nil
}

func defaultPlatformValueFallbackUserPrompt() string {
	return "Map the source attribute value to one existing platform value.\n\nAttribute ID: {{.AttrID}}\nDetected domain: {{.Domain}}\nProduct title: {{.ProductTitle}}\nSource value: {{.RawValue}}\n\nPlatform values:\n{{.PlatformValues}}\n\nSize chart:\n{{.SizeChart}}\n\nReturn JSON only."
}

func formatPlatformValuesForPrompt(values []string) string {
	if len(values) == 0 {
		return "(empty)"
	}
	var b strings.Builder
	for _, value := range values {
		b.WriteString("- ")
		b.WriteString(value)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
