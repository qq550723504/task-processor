package openai

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
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

// ValidGoogleInteractionsEndpoint confines live credentials to Google's native
// origin. A loopback origin is accepted solely for local transport fixtures.
func ValidGoogleInteractionsEndpoint(raw string) bool {
	if raw == googleInteractionsOrigin {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "http" && u.Host != "" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

func completeGoogleInteractionsOnce(ctx context.Context, config *ClientConfig, input TextCompletionRequest) (*TextCompletionResult, error) {
	if config.Model != "gemini-3.8-flash" || !ValidGoogleInteractionsEndpoint(config.BaseURL) || !validTextGenerationControls("google-interactions", input) {
		return nil, errors.Join(ErrTextNotDispatched, ErrTextInput)
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: config.Timeout, MaxResponseHeaderBytes: 32 << 10}
	defer transport.CloseIdleConnections()
	client := &boundedGoogleHTTPClient{client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: config.BaseURL}
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
	if err != nil || response == nil || response.Interaction == nil {
		if !client.dispatched {
			return nil, errors.Join(ErrTextNotDispatched, ErrTextInput)
		}
		return nil, ErrTextOutcomeUnknown
	}
	return mapGoogleInteraction(response.Interaction, config.Model, input.MaximumOutputTokens), nil
}

type boundedGoogleHTTPClient struct {
	client     *http.Client
	endpoint   string
	dispatched bool
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
		return nil, ErrTextOutcomeUnknown
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
		return nil, ErrTextOutcomeUnknown
	}
	bounded, err := io.ReadAll(io.LimitReader(response.Body, MaxTextResponseBytes+1))
	if err != nil || len(bounded) > MaxTextResponseBytes {
		return nil, ErrTextOutcomeUnknown
	}
	var metadata struct {
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(bounded, &metadata) != nil {
		return nil, ErrTextOutcomeUnknown
	}
	for name := range metadata.Usage {
		if strings.HasPrefix(name, "total_") && name != "total_input_tokens" && name != "total_output_tokens" && name != "total_thought_tokens" && name != "total_tokens" && name != "total_cached_tokens" && name != "total_tool_use_tokens" {
			return nil, ErrTextOutcomeUnknown
		}
	}
	response.Body = io.NopCloser(bytes.NewReader(bounded))
	response.ContentLength = int64(len(bounded))
	return response, nil
}

func mapGoogleInteraction(source *interaction.Interaction, expectedModel string, maximumOutputTokens int) *TextCompletionResult {
	result := &TextCompletionResult{}
	if source == nil || source.Model == nil || string(*source.Model) != expectedModel || source.Usage == nil || source.Usage.TotalInputTokens == nil || source.Usage.TotalOutputTokens == nil || source.Usage.TotalThoughtTokens == nil || source.Usage.TotalTokens == nil {
		return result
	}
	u := source.Usage
	input, output, thought, total := *u.TotalInputTokens, *u.TotalOutputTokens, *u.TotalThoughtTokens, *u.TotalTokens
	if input < 0 || output < 0 || thought < 0 || total <= 0 || input > total || output > total-input || thought != total-input-output || output+thought > maximumOutputTokens || u.TotalToolUseTokens == nil || *u.TotalToolUseTokens != 0 || len(u.ToolUseTokensByModality) != 0 || u.TotalCachedTokens != nil && (*u.TotalCachedTokens < 0 || *u.TotalCachedTokens > input) || len(u.GroundingToolCount) != 0 {
		return result
	}
	result.UsageKnown = true
	result.Usage = Usage{PromptTokens: input, CompletionTokens: output + thought, TotalTokens: total}
	if source.ID != nil {
		result.ID = *source.ID
	}
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
			finish = "other"
		}
	}
	if outputs != 1 || content == "" {
		finish = "other"
	}
	result.Choices = []ChatCompletionChoice{{Message: ChatCompletionMessage{Content: content}, FinishReason: finish}}
	return result
}
