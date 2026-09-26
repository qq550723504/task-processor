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

	goopenai "github.com/sashabaranov/go-openai"
)

const MaxTextPromptBytes = 128 << 10
const MaxTextResponseBytes = 256 << 10

var ErrTextInput = errors.New("invalid bounded text request")
var ErrTextOutcomeUnknown = errors.New("text provider outcome is unknown; do not redispatch")
var ErrTextNotDispatched = errors.New("text provider request was not dispatched")

// TextCompletionRequest contains text only. Routing, credentials and model are
// chosen by the current manager configuration, never by model-produced content.
// This transport seam is internal: callers must reserve usage and durably admit
// the invocation before calling it. It does not implement a second ledger.
type TextCompletionRequest struct {
	System, Prompt      string
	MaximumOutputTokens int
	// BeforeDispatch rechecks the consumer's live authorization after queueing.
	// It is an in-process callback, never serialized into provider input.
	BeforeDispatch func() error `json:"-"`
}

// UsageKnown requires each provider counter to be present and consistent. SDK
// integer zero values alone cannot distinguish missing usage from observed zero.
type TextCompletionResult struct {
	ChatCompletionResponse
	UsageKnown bool
}

// UsesOrganizationCredentials reports whether absent organization credentials
// fail closed instead of falling back to the registered global configuration.
func (m *Manager) UsesOrganizationCredentials() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	resolver, ok := m.configResolver.(*GormCredentialResolver)
	return ok && resolver != nil && resolver.organizationScope
}

func (m *Manager) ResolveTextRoute(ctx context.Context, name string) (EffectiveClientRoute, error) {
	resolved, err := m.resolveTextConfiguration(ctx, name)
	if err != nil {
		return EffectiveClientRoute{}, err
	}
	return resolved.route, nil
}

func (m *Manager) resolveTextConfiguration(ctx context.Context, name string) (effectiveClientConfiguration, error) {
	if err := ctx.Err(); err != nil {
		return effectiveClientConfiguration{}, err
	}
	resolved, err := m.resolveEffectiveClientConfiguration(ctx, name)
	if err != nil {
		return effectiveClientConfiguration{}, err
	}
	style := strings.ToLower(strings.TrimSpace(resolved.config.APIStyle))
	if style != "" && style != "openai" && style != "openai-compatible" && style != "grsai" {
		return effectiveClientConfiguration{}, ErrClientConfigurationUnsupported
	}
	if resolved.config.Timeout <= 0 || resolved.config.Timeout > 5*time.Minute {
		return effectiveClientConfiguration{}, ErrClientConfigurationUnavailable
	}
	// Existing default text configurations name the wire protocol as OpenAI.
	// Attribute these two documented GRSAI origins to the actual provider.
	if endpoint, err := url.Parse(resolved.config.BaseURL); err == nil && resolved.route.ProviderID == "openai" && (strings.EqualFold(endpoint.Hostname(), "grsaiapi.com") || strings.EqualFold(endpoint.Hostname(), "grsai.dakka.com.cn")) {
		resolved.route.ProviderID = "grsai"
	}
	return resolved, nil
}

// CompleteText re-resolves the current credential and compares the full quoted
// route. No legacy resolver-version alias, provider fallback, model override or
// automatic retry is accepted. Missing usage remains missing for the caller.
func (m *Manager) CompleteText(ctx context.Context, name string, expected EffectiveClientRoute, input TextCompletionRequest) (result *TextCompletionResult, resultErr error) {
	handedToTransport := false
	defer func() {
		if resultErr != nil && !handedToTransport {
			resultErr = errors.Join(ErrTextNotDispatched, resultErr)
		}
	}()
	if input.System == "" || input.Prompt == "" || input.MaximumOutputTokens <= 0 || input.MaximumOutputTokens > 65536 {
		return nil, ErrTextInput
	}
	wire, err := json.Marshal(input)
	if err != nil || len(wire) > MaxTextPromptBytes {
		return nil, ErrTextInput
	}
	resolved, err := m.resolveTextConfiguration(ctx, name)
	if err != nil {
		return nil, err
	}
	if expected != resolved.route {
		return nil, ErrClientConfigurationChanged
	}
	ctx, cancel := context.WithTimeout(ctx, resolved.config.Timeout)
	defer cancel()
	client, err := m.clientFromEffectiveConfiguration(resolved, name)
	if err != nil {
		return nil, err
	}
	// Reuse the current per-credential pool's rate and concurrency controls.
	pool := client.pool
	if err := pool.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	select {
	case pool.semaphore <- struct{}{}:
		defer func() { <-pool.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Waiting cannot freeze authorization to an obsolete configuration. Use the
	// newly resolved credential, not a cached client containing an earlier key.
	current, err := m.resolveTextConfiguration(ctx, name)
	if err != nil {
		return nil, err
	}
	if current.route != expected {
		return nil, ErrClientConfigurationChanged
	}
	if input.BeforeDispatch != nil {
		if err := input.BeforeDispatch(); err != nil {
			return nil, err
		}
	}
	handedToTransport = true
	return completeTextOnce(ctx, current.config, input)
}

func completeTextOnce(ctx context.Context, config *ClientConfig, input TextCompletionRequest) (*TextCompletionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	// A dedicated HTTP/1 connection has no stale keep-alive or HTTP/2 replay
	// path. SDK, application and HTTP redirects all have zero retries here.
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: config.Timeout, MaxResponseHeaderBytes: 32 << 10}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	sdkConfig := goopenai.DefaultConfig(config.APIKey)
	sdkConfig.BaseURL = config.BaseURL
	capture := &boundedTextHTTPClient{client: httpClient}
	sdkConfig.HTTPClient = capture
	sdk := goopenai.NewClientWithConfig(sdkConfig)
	response, err := sdk.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model: config.Model, MaxTokens: input.MaximumOutputTokens, Stream: false,
		Messages: []goopenai.ChatCompletionMessage{{Role: "system", Content: input.System}, {Role: "user", Content: input.Prompt}},
	})
	if err != nil {
		// Raw provider error bodies/URLs can include sensitive input. Only a safe
		// classification leaves this seam; dispatch/usage ownership stays upstream.
		if !capture.dispatched {
			return nil, errors.Join(ErrTextNotDispatched, ErrTextInput)
		}
		return nil, ErrTextOutcomeUnknown
	}
	return &TextCompletionResult{ChatCompletionResponse: *convertResponse(&response), UsageKnown: capture.usageKnown}, nil
}

type boundedTextHTTPClient struct {
	client     *http.Client
	usageKnown bool
	dispatched bool
}

func (c *boundedTextHTTPClient) Do(request *http.Request) (*http.Response, error) {
	// GRSAI requires an explicit stream boolean. The existing SDK marks false
	// omitempty, so add only this wire field while retaining SDK serialization,
	// authentication and response handling. Bound the actual outbound envelope.
	input, err := io.ReadAll(io.LimitReader(request.Body, MaxTextPromptBytes+1))
	_ = request.Body.Close()
	if err != nil || len(input) > MaxTextPromptBytes {
		return nil, ErrTextInput
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(input, &envelope) != nil || envelope == nil {
		return nil, ErrTextInput
	}
	envelope["stream"] = json.RawMessage("false")
	wire, err := json.Marshal(envelope)
	if err != nil || len(wire) > MaxTextPromptBytes {
		return nil, ErrTextInput
	}
	request.Body = io.NopCloser(bytes.NewReader(wire))
	request.ContentLength = int64(len(wire))
	request.GetBody = nil
	// Once handed to net/http, even a network error is conservatively unknown.
	c.dispatched = true
	response, err := c.client.Do(request)
	if err != nil {
		return nil, ErrTextOutcomeUnknown
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrTextOutcomeUnknown
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxTextResponseBytes+1))
	if err != nil || len(body) > MaxTextResponseBytes {
		return nil, ErrTextOutcomeUnknown
	}
	var presence struct {
		Usage *struct {
			Prompt     *int `json:"prompt_tokens"`
			Completion *int `json:"completion_tokens"`
			Total      *int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &presence); err != nil {
		return nil, ErrTextOutcomeUnknown
	}
	u := presence.Usage
	if u != nil && u.Prompt != nil && u.Completion != nil && u.Total != nil {
		c.usageKnown = *u.Total > 0 && *u.Prompt >= 0 && *u.Completion >= 0 && *u.Prompt <= *u.Total && *u.Completion == *u.Total-*u.Prompt
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, nil
}
