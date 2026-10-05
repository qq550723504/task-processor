package openai

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	google "google.golang.org/genai/interactions"
	"google.golang.org/genai/interactions/models/components"
	interaction "google.golang.org/genai/interactions/models/interactions"
	"google.golang.org/genai/interactions/models/operations"
	"google.golang.org/genai/interactions/retry"
)

const googleInteractionsOrigin = "https://generativelanguage.googleapis.com"

// ValidGoogleInteractionsEndpoint confines deployed credentials to Google's
// exact native origin. Tests inject a transport below URL admission.
func ValidGoogleInteractionsEndpoint(raw string) bool {
	return raw == googleInteractionsOrigin
}

func completeGoogleInteractionsOnce(ctx context.Context, config *ClientConfig, input TextCompletionRequest) (*TextCompletionResult, error) {
	if config.Model != "gemini-3.8-flash" || !ValidGoogleInteractionsEndpoint(config.BaseURL) || !validTextGenerationControls("google-interactions", input) {
		return nil, errors.Join(ErrTextNotDispatched, ErrTextInput)
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: config.Timeout, MaxResponseHeaderBytes: 32 << 10}
	defer transport.CloseIdleConnections()
	var wireTransport http.RoundTripper = transport
	if config.GoogleInteractionsFixtureTransport != nil {
		wireTransport = config.GoogleInteractionsFixtureTransport
	}
	client := &boundedGoogleHTTPClient{client: &http.Client{Transport: wireTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: config.BaseURL}
	sdk := google.New(google.WithServerURL(config.BaseURL), google.WithAPIVersion("v1beta"), google.WithSecurity(components.Security{APIKey: google.String(config.APIKey)}), google.WithClient(client), google.WithRetryConfig(retry.Config{Strategy: "none"}))
	modelInput := interaction.NewInteractionsInput(input.Prompt)
	request := interaction.CreateModelInteraction{
		Model:             interaction.ModelGemini38Flash,
		Input:             &modelInput,
		SystemInstruction: google.String(input.System),
		Background:        google.Bool(false), Store: google.Bool(false), Stream: google.Bool(false),
		GenerationConfig: &interaction.GenerationConfig{MaxOutputTokens: google.Int(input.MaximumOutputTokens), ThinkingLevel: interaction.ThinkingLevelLow.ToPointer()},
	}
	response, err := sdk.Interactions.Create(ctx, operations.CreateInteractionRequest{Body: operations.NewCreateInteractionRequestBody(request)}, operations.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		if !client.dispatched {
			return nil, errors.Join(ErrTextNotDispatched, ErrTextInput)
		}
		code := client.diagnosticCode
		if code == "" {
			code = "provider_response_decode"
		}
		return nil, textOutcomeDiagnostic{code: code}
	}
	if response == nil || response.Interaction == nil {
		return nil, textOutcomeDiagnostic{code: "provider_response_empty"}
	}
	return mapGoogleInteraction(response.Interaction, config.Model, input.MaximumOutputTokens), nil
}

type boundedGoogleHTTPClient struct {
	client     *http.Client
	endpoint   string
	dispatched bool
	// Fixed diagnostic only; never retain provider response bodies or SDK errors.
	diagnosticCode string
}

func (c *boundedGoogleHTTPClient) Do(request *http.Request) (*http.Response, error) {
	base, err := url.Parse(c.endpoint)
	if err != nil || request.Method != http.MethodPost || request.URL.Scheme != base.Scheme || request.URL.Host != base.Host || request.URL.Path != "/v1beta/interactions" || request.URL.RawQuery != "" || request.URL.Fragment != "" {
		return nil, ErrTextInput
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, MaxTextPromptBytes+1))
	_ = request.Body.Close()
	if err != nil || len(body) > MaxTextPromptBytes {
		return nil, ErrTextInput
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || envelope == nil {
		return nil, ErrTextInput
	}
	for _, field := range []string{"store", "background", "stream"} {
		if !bytes.Equal(bytes.TrimSpace(envelope[field]), []byte("false")) {
			return nil, ErrTextInput
		}
	}
	for _, field := range []string{"tools", "previous_interaction_id", "agent", "environment", "cached_content"} {
		if _, exists := envelope[field]; exists {
			return nil, ErrTextInput
		}
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	// net/http may replay a POST with GetBody on a stale connection. This request
	// has a fresh HTTP/1 connection, and no replay body even if that changes.
	request.GetBody = nil
	c.dispatched = true
	response, err := c.client.Do(request)
	if err != nil {
		c.diagnosticCode = googleTransportDiagnostic(err, request.Context())
		return nil, ErrTextOutcomeUnknown
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode >= 300 && response.StatusCode <= 599 {
			c.diagnosticCode = fmt.Sprintf("provider_http_%d", response.StatusCode)
		} else {
			c.diagnosticCode = "provider_http_unexpected"
		}
		return nil, ErrTextOutcomeUnknown
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
		c.diagnosticCode = "provider_response_content_type"
		return nil, ErrTextOutcomeUnknown
	}
	bounded, err := io.ReadAll(io.LimitReader(response.Body, MaxTextResponseBytes+1))
	if err != nil || len(bounded) > MaxTextResponseBytes {
		c.diagnosticCode = "provider_response_read"
		return nil, ErrTextOutcomeUnknown
	}
	var metadata struct {
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(bounded, &metadata) != nil {
		c.diagnosticCode = "provider_response_json"
		return nil, ErrTextOutcomeUnknown
	}
	for name := range metadata.Usage {
		if strings.HasPrefix(name, "total_") && name != "total_input_tokens" && name != "total_output_tokens" && name != "total_thought_tokens" && name != "total_tokens" && name != "total_cached_tokens" && name != "total_tool_use_tokens" {
			c.diagnosticCode = "provider_usage_unpriced_dimension"
			return nil, ErrTextOutcomeUnknown
		}
	}
	response.Body = io.NopCloser(bytes.NewReader(bounded))
	response.ContentLength = int64(len(bounded))
	return response, nil
}

func googleTransportDiagnostic(err error, ctx context.Context) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "provider_transport_timeout"
	case errors.Is(err, context.Canceled), errors.Is(ctx.Err(), context.Canceled):
		return "provider_transport_canceled"
	default:
		return "provider_transport_error"
	}
}

func mapGoogleInteraction(source *interaction.Interaction, expectedModel string, maximumOutputTokens int) *TextCompletionResult {
	result := &TextCompletionResult{}
	if source != nil && source.ID != nil {
		result.ID = *source.ID
	}
	if source == nil || source.Model == nil || string(*source.Model) != expectedModel {
		result.OutcomeDiagnostic = "provider_model_mismatch"
		return result
	}
	if source.Usage == nil || source.Usage.TotalInputTokens == nil || source.Usage.TotalOutputTokens == nil || source.Usage.TotalThoughtTokens == nil || source.Usage.TotalTokens == nil || source.Usage.TotalToolUseTokens == nil {
		result.OutcomeDiagnostic = "provider_usage_missing"
		return result
	}
	u := source.Usage
	input, output, thought, total := *u.TotalInputTokens, *u.TotalOutputTokens, *u.TotalThoughtTokens, *u.TotalTokens
	if input < 0 || output < 0 || thought < 0 || total <= 0 || input > total || output > total-input || thought != total-input-output || output+thought > maximumOutputTokens {
		result.OutcomeDiagnostic = "provider_usage_inconsistent"
		return result
	}
	if *u.TotalToolUseTokens != 0 || len(u.ToolUseTokensByModality) != 0 || len(u.GroundingToolCount) != 0 {
		result.OutcomeDiagnostic = "provider_usage_unpriced_dimension"
		return result
	}
	if u.TotalCachedTokens != nil && (*u.TotalCachedTokens < 0 || *u.TotalCachedTokens > input) {
		result.OutcomeDiagnostic = "provider_usage_inconsistent"
		return result
	}
	if !textOnlyModalityBreakdown(u.InputTokensByModality, input) || !textOnlyModalityBreakdown(u.OutputTokensByModality, output) {
		result.OutcomeDiagnostic = "provider_usage_unpriced_dimension"
		return result
	}
	if u.CachedTokensByModality != nil && (u.TotalCachedTokens == nil || !textOnlyModalityBreakdown(u.CachedTokensByModality, *u.TotalCachedTokens)) {
		result.OutcomeDiagnostic = "provider_usage_unpriced_dimension"
		return result
	}
	result.UsageKnown = true
	result.Usage = Usage{PromptTokens: input, CompletionTokens: output + thought, TotalTokens: total}
	result.Model = expectedModel
	finish := "other"
	if source.Status == interaction.InteractionStatusCompleted && len(source.Errors) == 0 {
		finish = "stop"
	}
	var content string
	var outputs int
	for _, step := range source.Steps {
		switch step.Type {
		case interaction.StepTypeThought:
			// Never include hidden reasoning in actions, logs or review facts.
		case interaction.StepTypeModelOutput:
			outputs++
			if step.ModelOutputStep == nil || step.ModelOutputStep.Error != nil || len(step.ModelOutputStep.Content) != 1 || step.ModelOutputStep.Content[0].Type != interaction.ContentTypeText || step.ModelOutputStep.Content[0].TextContent == nil {
				finish = "other"
				continue
			}
			content = step.ModelOutputStep.Content[0].TextContent.Text
		default:
			// An unrequested tool or unknown step can add an unpriced external
			// dimension. Keep the existing invocation UNKNOWN.
			result.UsageKnown = false
			result.OutcomeDiagnostic = "provider_step_unexpected"
			finish = "other"
		}
	}
	if outputs != 1 || content == "" {
		finish = "other"
	}
	result.Choices = []ChatCompletionChoice{{Message: ChatCompletionMessage{Content: content}, FinishReason: finish}}
	return result
}

func textOnlyModalityBreakdown(rows []interaction.ModalityTokens, expected int) bool {
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
