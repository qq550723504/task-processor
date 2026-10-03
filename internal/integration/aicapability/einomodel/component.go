// Package einomodel is the bounded Eino component and single-send transport
// seam shared by Chat planning and Product title execution.
package einomodel

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
	"sync"
	"sync/atomic"
	"time"

	claude "github.com/cloudwego/eino-ext/components/model/claude"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type AdapterKind string

const (
	AdapterOpenAICompatible AdapterKind = "openai-compatible"
	AdapterClaudeNative     AdapterKind = "claude-native"
)

var (
	ErrInvalid = errors.New("invalid bounded model configuration or output")
	// ErrObservedInvalidSettled is emitted only after the terminal recorder
	// accepted observed usage and its commercial settlement returned success.
	ErrObservedInvalidSettled = errors.New("observed invalid model output settled")
	ErrNotDispatched          = errors.New("model request not dispatched")
	ErrOutcomeUnknown         = errors.New("model outcome unknown; do not redispatch")
	ErrUsageUnknown           = errors.New("provider usage unknown")
)

type ComponentConfig struct {
	Adapter             AdapterKind
	Endpoint            string
	APIKey              string
	ModelID             string
	MaximumOutputTokens int
}

// NewComponent always constructs an unbound model instance. Neither callers
// nor model output may supply Eino model.Option or a tool schema.
func NewComponent(ctx context.Context, cfg ComponentConfig, guard *GuardedClient) (model.BaseChatModel, error) {
	if ctx == nil || guard == nil || guard.Client == nil || cfg.APIKey == "" || cfg.ModelID == "" ||
		cfg.MaximumOutputTokens < 1 || cfg.MaximumOutputTokens > 65536 ||
		!validEndpoint(cfg.Endpoint) || cfg.Adapter != guard.cfg.Adapter ||
		cfg.Endpoint != guard.cfg.Endpoint || cfg.ModelID != guard.cfg.ModelID ||
		cfg.MaximumOutputTokens != guard.cfg.MaximumOutputTokens {
		return nil, ErrInvalid
	}
	maxTokens := cfg.MaximumOutputTokens
	switch cfg.Adapter {
	case AdapterOpenAICompatible:
		return openaimodel.NewChatModel(ctx, &openaimodel.ChatModelConfig{
			APIKey: cfg.APIKey, BaseURL: cfg.Endpoint, Model: cfg.ModelID,
			MaxTokens: &maxTokens, HTTPClient: guard.Client,
		})
	case AdapterClaudeNative:
		endpoint := cfg.Endpoint
		return claude.NewChatModel(ctx, &claude.Config{
			APIKey: cfg.APIKey, BaseURL: &endpoint, Model: cfg.ModelID,
			MaxTokens: maxTokens, HTTPClient: guard.Client,
		})
	default:
		return nil, ErrInvalid
	}
}

type GuardConfig struct {
	Adapter              AdapterKind
	Endpoint             string
	ModelID              string
	MaximumOutputTokens  int
	MaximumRequestBytes  int64
	MaximumResponseBytes int64
	Timeout              time.Duration
}

type ObservedUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Known            bool
}

type GuardedClient struct {
	Client    *http.Client
	cfg       GuardConfig
	endpoint  *url.URL
	base      http.RoundTripper
	finalGate func(context.Context) error
	sent      atomic.Bool
	sendMu    sync.Mutex
	mu        sync.Mutex
	usage     ObservedUsage
}

func validEndpoint(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

// ValidEndpoint uses the same endpoint admission rule as the final transport.
func ValidEndpoint(raw string) bool { return validEndpoint(raw) }

// NewGuardedClient creates a per-invocation transport. Even if an SDK retries,
// follows a redirect, or Generate is called twice, only one underlying model
// RoundTrip can occur. finalGate runs at the actual HTTP handoff.
func NewGuardedClient(cfg GuardConfig, base http.RoundTripper, finalGate func(context.Context) error) (*GuardedClient, error) {
	if (cfg.Adapter != AdapterOpenAICompatible && cfg.Adapter != AdapterClaudeNative) ||
		!validEndpoint(cfg.Endpoint) || cfg.ModelID == "" || cfg.MaximumOutputTokens < 1 ||
		cfg.MaximumOutputTokens > 65536 || cfg.MaximumRequestBytes < 1 ||
		cfg.MaximumRequestBytes > 1<<20 || cfg.MaximumResponseBytes < 1 ||
		cfg.MaximumResponseBytes > 1<<20 || cfg.Timeout <= 0 || cfg.Timeout > 2*time.Minute ||
		finalGate == nil {
		return nil, ErrInvalid
	}
	endpoint, _ := url.Parse(cfg.Endpoint)
	if base == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableKeepAlives = true
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		base = transport
	}
	guard := &GuardedClient{cfg: cfg, endpoint: endpoint, base: base, finalGate: finalGate}
	guard.Client = &http.Client{Transport: guard, Timeout: cfg.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return guard, nil
}

func (g *GuardedClient) NetworkSends() int {
	if g == nil || !g.sent.Load() {
		return 0
	}
	return 1
}

func (g *GuardedClient) Usage() ObservedUsage {
	if g == nil {
		return ObservedUsage{}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.usage
}

func (g *GuardedClient) RoundTrip(req *http.Request) (*http.Response, error) {
	if g == nil || req == nil || req.URL == nil || req.Method != http.MethodPost ||
		req.URL.Scheme != g.endpoint.Scheme || !strings.EqualFold(req.URL.Host, g.endpoint.Host) ||
		!strings.HasPrefix(req.URL.EscapedPath(), strings.TrimSuffix(g.endpoint.EscapedPath(), "/")+"/") ||
		req.URL.RawQuery != "" || req.Body == nil {
		return nil, ErrNotDispatched
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, g.cfg.MaximumRequestBytes+1))
	_ = req.Body.Close()
	if err != nil || int64(len(body)) > g.cfg.MaximumRequestBytes || !validWire(body, g.cfg) {
		return nil, ErrNotDispatched
	}
	g.sendMu.Lock()
	if g.sent.Load() {
		g.sendMu.Unlock()
		return nil, ErrOutcomeUnknown
	}
	if err := g.finalGate(req.Context()); err != nil {
		g.sendMu.Unlock()
		return nil, errors.Join(ErrNotDispatched, err)
	}
	if !g.sent.CompareAndSwap(false, true) {
		g.sendMu.Unlock()
		return nil, ErrOutcomeUnknown
	}
	g.sendMu.Unlock()
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = nil
	response, err := g.base.RoundTrip(req)
	if err != nil {
		return nil, ErrOutcomeUnknown
	}
	if response == nil || response.Body == nil {
		return nil, ErrOutcomeUnknown
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, g.cfg.MaximumResponseBytes+1))
	_ = response.Body.Close()
	if err != nil || int64(len(raw)) > g.cfg.MaximumResponseBytes {
		return nil, ErrOutcomeUnknown
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		usage := observeUsage(raw, g.cfg.Adapter)
		g.mu.Lock()
		g.usage = usage
		g.mu.Unlock()
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	response.ContentLength = int64(len(raw))
	return response, nil
}

func validWire(raw []byte, cfg GuardConfig) bool {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil || envelope == nil {
		return false
	}
	var modelID string
	if json.Unmarshal(envelope["model"], &modelID) != nil || modelID != cfg.ModelID {
		return false
	}
	if _, found := envelope["tools"]; found {
		return false
	}
	if _, found := envelope["tool_choice"]; found {
		return false
	}
	if stream, found := envelope["stream"]; found && string(stream) != "false" {
		return false
	}
	var max int
	if cfg.Adapter == AdapterClaudeNative {
		if json.Unmarshal(envelope["max_tokens"], &max) != nil {
			return false
		}
	} else if value, found := envelope["max_completion_tokens"]; found {
		if json.Unmarshal(value, &max) != nil {
			return false
		}
	} else if json.Unmarshal(envelope["max_tokens"], &max) != nil {
		return false
	}
	return max > 0 && max <= cfg.MaximumOutputTokens
}

func observeUsage(raw []byte, adapter AdapterKind) ObservedUsage {
	if adapter == AdapterOpenAICompatible {
		var result struct {
			Usage *struct {
				Prompt     *int `json:"prompt_tokens"`
				Completion *int `json:"completion_tokens"`
				Total      *int `json:"total_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(raw, &result) != nil || result.Usage == nil ||
			result.Usage.Prompt == nil || result.Usage.Completion == nil || result.Usage.Total == nil {
			return ObservedUsage{}
		}
		p, c, t := *result.Usage.Prompt, *result.Usage.Completion, *result.Usage.Total
		if p < 0 || c < 0 || t <= 0 || p+c != t {
			return ObservedUsage{}
		}
		return ObservedUsage{PromptTokens: p, CompletionTokens: c, TotalTokens: t, Known: true}
	}
	var result struct {
		Usage *struct {
			Input      *int `json:"input_tokens"`
			Output     *int `json:"output_tokens"`
			CacheRead  int  `json:"cache_read_input_tokens"`
			CacheWrite int  `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Usage == nil ||
		result.Usage.Input == nil || result.Usage.Output == nil {
		return ObservedUsage{}
	}
	// V1's two-category tariff has no cache rates. A cache-bearing native
	// response remains unknown until an admitted cost policy can map it.
	u := result.Usage
	if *u.Input < 0 || *u.Output < 0 || u.CacheRead != 0 || u.CacheWrite != 0 ||
		*u.Input+*u.Output <= 0 {
		return ObservedUsage{}
	}
	return ObservedUsage{PromptTokens: *u.Input, CompletionTokens: *u.Output,
		TotalTokens: *u.Input + *u.Output, Known: true}
}

type TextOutput struct {
	Content      string
	FinishReason string
	Usage        ObservedUsage
}

// GenerateText makes one non-streaming, no-tool call. Raw counter presence is
// supplied by the guarded transport, not inferred from Eino's optional ints.
func GenerateText(ctx context.Context, component model.BaseChatModel, system, prompt string, guard *GuardedClient) (TextOutput, error) {
	if ctx == nil || component == nil || guard == nil || system == "" || prompt == "" {
		return TextOutput{}, ErrInvalid
	}
	message, err := component.Generate(ctx, []*schema.Message{schema.SystemMessage(system), schema.UserMessage(prompt)})
	usage := guard.Usage()
	if err != nil {
		if guard.NetworkSends() == 0 {
			return TextOutput{}, errors.Join(ErrNotDispatched, err)
		}
		return TextOutput{Usage: usage}, ErrOutcomeUnknown
	}
	output := TextOutput{Usage: usage}
	if message != nil && message.ResponseMeta != nil {
		output.Content = message.Content
		output.FinishReason = message.ResponseMeta.FinishReason
	}
	if !usage.Known {
		return output, ErrUsageUnknown
	}
	if message == nil || message.Role != schema.Assistant || message.ResponseMeta == nil ||
		message.ResponseMeta.Usage == nil || message.ResponseMeta.Usage.PromptTokens != usage.PromptTokens ||
		message.ResponseMeta.Usage.CompletionTokens != usage.CompletionTokens ||
		message.ResponseMeta.Usage.TotalTokens != usage.TotalTokens ||
		len(message.ToolCalls) != 0 || message.ToolCallID != "" ||
		len(message.MultiContent) != 0 || len(message.AssistantGenMultiContent) != 0 ||
		message.ReasoningContent != "" || !safeComponentMetadata(message.Extra) ||
		message.Content == "" || len(message.Content) > 16384 ||
		(output.FinishReason != "stop" && output.FinishReason != "end_turn") {
		return output, ErrInvalid
	}
	return output, nil
}

func safeComponentMetadata(extra map[string]any) bool {
	for key, value := range extra {
		// The pinned OpenAI component records its request ID in Extra. It is
		// metadata, never text for the planner or title consumer.
		if key != "openai-request-id" || len(fmt.Sprint(value)) > 128 {
			return false
		}
	}
	return true
}
