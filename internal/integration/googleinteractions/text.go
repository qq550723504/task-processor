// Package googleinteractions extracts the stateless official SDK protocol from
// #586. Callers supply their own guarded client and own all business facts.
package googleinteractions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	google "google.golang.org/genai/interactions"
	"google.golang.org/genai/interactions/models/components"
	interaction "google.golang.org/genai/interactions/models/interactions"
	"google.golang.org/genai/interactions/models/operations"
	"google.golang.org/genai/interactions/retry"
)

const (
	Origin               = "https://generativelanguage.googleapis.com"
	Model                = "gemini-3.8-flash"
	AdapterPolicyVersion = "google-interactions-v1"
	UsageMappingVersion  = "google-interactions-usage-v1"
)

type Result struct {
	ID, Model, Content, FinishReason, Diagnostic string
	PromptTokens, CompletionTokens, TotalTokens  int
	UsageKnown                                   bool
}

func ValidEndpoint(raw string) bool { return raw == Origin }

// Generate performs a single SDK operation; only the supplied client may send.
// Neither the helper nor the SDK gets a second credential/route/retry owner.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

func Generate(ctx context.Context, client HTTPClient, key, system, prompt string, maxTokens int) (Result, error) {
	if ctx == nil || client == nil || key == "" || system == "" || prompt == "" || maxTokens < 1 || maxTokens > 65536 {
		return Result{}, errors.New("invalid Google text call")
	}
	sdk := google.New(google.WithServerURL(Origin), google.WithAPIVersion("v1beta"),
		google.WithSecurity(components.Security{APIKey: google.String(key)}), google.WithClient(client),
		google.WithRetryConfig(retry.Config{Strategy: "none"}))
	input := interaction.NewInteractionsInput(prompt)
	request := interaction.CreateModelInteraction{Model: interaction.ModelGemini38Flash, Input: &input,
		SystemInstruction: google.String(system), Background: google.Bool(false), Store: google.Bool(false), Stream: google.Bool(false),
		GenerationConfig: &interaction.GenerationConfig{MaxOutputTokens: google.Int(maxTokens), ThinkingLevel: interaction.ThinkingLevelLow.ToPointer()}}
	response, err := sdk.Interactions.Create(ctx, operations.CreateInteractionRequest{Body: operations.NewCreateInteractionRequestBody(request)},
		operations.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil || response == nil || response.Interaction == nil {
		// SDK errors may contain the raw upstream body or credential-bearing URL.
		return Result{}, errors.New("Google text outcome unavailable")
	}
	return mapInteraction(response.Interaction, maxTokens), nil
}

// Observe independently checks raw counter presence and unpriced dimensions
// before optional SDK/Eino integer defaults could conceal absent metadata.
func Observe(raw []byte, maxTokens int) Result {
	var metadata struct {
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &metadata) != nil || metadata.Usage == nil {
		return Result{Diagnostic: "provider_usage_missing"}
	}
	for name := range metadata.Usage {
		if strings.HasPrefix(name, "total_") && name != "total_input_tokens" && name != "total_output_tokens" &&
			name != "total_thought_tokens" && name != "total_tokens" && name != "total_cached_tokens" && name != "total_tool_use_tokens" {
			return Result{Diagnostic: "provider_usage_unpriced_dimension"}
		}
	}
	var source interaction.Interaction
	if json.Unmarshal(raw, &source) != nil {
		return Result{Diagnostic: "provider_response_decode"}
	}
	return mapInteraction(&source, maxTokens)
}

// SafeRequestID is opaque metadata, never a URL, provider body or recovery key.
func SafeRequestID(raw string) string {
	if len(raw) == 0 || len(raw) > 128 {
		return ""
	}
	for _, c := range raw {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return ""
		}
	}
	return raw
}

func mapInteraction(source *interaction.Interaction, maxTokens int) Result {
	result := Result{}
	if source != nil && source.ID != nil {
		result.ID = SafeRequestID(*source.ID)
	}
	if source == nil || source.Model == nil || string(*source.Model) != Model {
		result.Diagnostic = "provider_model_mismatch"
		return result
	}
	if source.OutputImage != nil || source.OutputAudio != nil || source.OutputVideo != nil {
		result.Diagnostic = "provider_step_unexpected"
		return result
	}
	if source.Usage == nil || source.Usage.TotalInputTokens == nil || source.Usage.TotalOutputTokens == nil ||
		source.Usage.TotalThoughtTokens == nil || source.Usage.TotalTokens == nil || source.Usage.TotalToolUseTokens == nil {
		result.Diagnostic = "provider_usage_missing"
		return result
	}
	u := source.Usage
	input, output, thought, total := *u.TotalInputTokens, *u.TotalOutputTokens, *u.TotalThoughtTokens, *u.TotalTokens
	if input < 0 || output < 0 || thought < 0 || total <= 0 || input > total || output > total-input ||
		thought != total-input-output || output+thought > maxTokens {
		result.Diagnostic = "provider_usage_inconsistent"
		return result
	}
	if *u.TotalToolUseTokens != 0 || len(u.ToolUseTokensByModality) != 0 || len(u.GroundingToolCount) != 0 {
		result.Diagnostic = "provider_usage_unpriced_dimension"
		return result
	}
	if u.TotalCachedTokens != nil && (*u.TotalCachedTokens < 0 || *u.TotalCachedTokens > input) {
		result.Diagnostic = "provider_usage_inconsistent"
		return result
	}
	if !textOnlyModality(u.InputTokensByModality, input) || !textOnlyModality(u.OutputTokensByModality, output) ||
		u.CachedTokensByModality != nil && (u.TotalCachedTokens == nil || !textOnlyModality(u.CachedTokensByModality, *u.TotalCachedTokens)) {
		result.Diagnostic = "provider_usage_unpriced_dimension"
		return result
	}
	result.UsageKnown = true
	result.PromptTokens, result.CompletionTokens, result.TotalTokens = input, output+thought, total
	result.Model, result.FinishReason = Model, "other"
	if source.Status == interaction.InteractionStatusCompleted && len(source.Errors) == 0 {
		result.FinishReason = "stop"
	}
	outputs := 0
	for _, step := range source.Steps {
		switch step.Type {
		case interaction.StepTypeThought: // Never export hidden thought content.
		case interaction.StepTypeModelOutput:
			outputs++
			if step.ModelOutputStep == nil {
				result.FinishReason = "other"
				continue
			}
			for _, content := range step.ModelOutputStep.Content {
				if content.Type != interaction.ContentTypeText || content.TextContent == nil {
					result.UsageKnown, result.Diagnostic = false, "provider_step_unexpected"
					result.Content, result.FinishReason = "", "other"
					return result
				}
			}
			if step.ModelOutputStep.Error != nil || len(step.ModelOutputStep.Content) != 1 {
				result.FinishReason = "other"
				continue
			}
			result.Content = step.ModelOutputStep.Content[0].TextContent.Text
		default:
			result.UsageKnown, result.Diagnostic = false, "provider_step_unexpected"
			result.Content, result.FinishReason = "", "other"
			return result
		}
	}
	if outputs != 1 || result.Content == "" {
		result.FinishReason = "other"
	}
	return result
}

func textOnlyModality(rows []interaction.ModalityTokens, expected int) bool {
	if rows == nil {
		return true
	}
	remaining := expected
	for _, row := range rows {
		if row.Modality == nil || *row.Modality != interaction.ResponseModalityText || row.Tokens == nil || *row.Tokens < 0 || *row.Tokens > remaining {
			return false
		}
		remaining -= *row.Tokens
	}
	return remaining == 0
}
