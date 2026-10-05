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

	"task-processor/internal/integration/googleinteractions"
)

const googleInteractionsOrigin = googleinteractions.Origin

func ValidGoogleInteractionsEndpoint(raw string) bool { return googleinteractions.ValidEndpoint(raw) }

// The retained generic SDK API reuses the extracted stateless protocol helper.
// Product title execution uses the shared governed Eino component directly.
func completeGoogleInteractionsOnce(ctx context.Context, config *ClientConfig, input TextCompletionRequest) (*TextCompletionResult, error) {
	if config.Model != googleinteractions.Model || !ValidGoogleInteractionsEndpoint(config.BaseURL) || !validTextGenerationControls("google-interactions", input) {
		return nil, errors.Join(ErrTextNotDispatched, ErrTextInput)
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: config.Timeout, MaxResponseHeaderBytes: 32 << 10}
	defer transport.CloseIdleConnections()
	var wireTransport http.RoundTripper = transport
	if config.GoogleInteractionsFixtureTransport != nil {
		wireTransport = config.GoogleInteractionsFixtureTransport
	}
	client := &boundedGoogleHTTPClient{client: &http.Client{Transport: wireTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: config.BaseURL}
	parsed, err := googleinteractions.Generate(ctx, client, config.APIKey, input.System, input.Prompt, input.MaximumOutputTokens)
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
	result := &TextCompletionResult{UsageKnown: parsed.UsageKnown, OutcomeDiagnostic: parsed.Diagnostic}
	result.ID, result.Model = parsed.ID, parsed.Model
	result.Usage = Usage{PromptTokens: parsed.PromptTokens, CompletionTokens: parsed.CompletionTokens, TotalTokens: parsed.TotalTokens}
	result.Choices = []ChatCompletionChoice{{Message: ChatCompletionMessage{Content: parsed.Content}, FinishReason: parsed.FinishReason}}
	return result, nil
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
