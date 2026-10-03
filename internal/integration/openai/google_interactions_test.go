package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func googleTextTestManager(t *testing.T, endpoint string) *Manager {
	t.Helper()
	m, err := NewManager(&ManagerConfig{Clients: map[string]*ClientConfig{"text": {
		APIKey: "fixture-key", Model: "gemini-3.8-flash", APIStyle: "google-interactions", BaseURL: endpoint, Timeout: time.Second,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func googleTextTestRequest() TextCompletionRequest {
	return TextCompletionRequest{System: "Return JSON.", Prompt: "synthetic item", MaximumOutputTokens: 128, OutputLimitField: "max_output_tokens", ThinkingLevel: "low"}
}

func TestGoogleInteractionsSingleStatelessWireAndThoughtUsage(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		config, _ := payload["generation_config"].(map[string]any)
		if r.Method != http.MethodPost || r.URL.Path != "/v1beta/interactions" || r.Header.Get("x-goog-api-key") != "fixture-key" || payload["model"] != "gemini-3.8-flash" || payload["system_instruction"] != "Return JSON." || payload["input"] != "synthetic item" || payload["store"] != false || payload["background"] != false || payload["stream"] != false || payload["tools"] != nil || payload["previous_interaction_id"] != nil || config["thinking_level"] != "low" || config["max_output_tokens"] != float64(128) {
			t.Errorf("unexpected Google request path=%q payload=%#v", r.URL.Path, payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"interaction-1","model":"gemini-3.8-flash","status":"completed","steps":[{"type":"thought","content":[]},{"type":"model_output","content":[{"type":"text","text":"{\"Kind\":\"interrupt\"}"}]}],"usage":{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":3}],"output_tokens_by_modality":[{"modality":"text","tokens":4}]}}`))
	}))
	defer srv.Close()
	m := googleTextTestManager(t, srv.URL)
	route, err := m.ResolveTextRoute(context.Background(), "text")
	if err != nil || route.ProviderID != "google" {
		t.Fatalf("route=%+v err=%v", route, err)
	}
	resp, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
	if err != nil || resp == nil || !resp.UsageKnown || resp.ID != "interaction-1" || resp.Usage.PromptTokens != 3 || resp.Usage.CompletionTokens != 11 || resp.Usage.TotalTokens != 14 || len(resp.Choices) != 1 || resp.Choices[0].FinishReason != "stop" || resp.Choices[0].Message.Content != `{"Kind":"interrupt"}` || calls.Load() != 1 {
		t.Fatalf("response=%+v err=%v calls=%d", resp, err, calls.Load())
	}
}

func TestGoogleInteractionsMissingOrUnpriceableUsageFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, usage string }{
		{"missing thoughts", `{"total_input_tokens":3,"total_output_tokens":4,"total_tokens":7}`},
		{"missing tool counter", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14}`},
		{"inconsistent", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":13}`},
		{"tool use", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":15,"total_tool_use_tokens":1}`},
		{"tool modality despite zero", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"tool_use_tokens_by_modality":[{"modality":"text","tokens":1}]}`},
		{"image input", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":2},{"modality":"image","tokens":1}]}`},
		{"audio output", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"output_tokens_by_modality":[{"modality":"text","tokens":3},{"modality":"audio","tokens":1}]}`},
		{"image cache", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"total_cached_tokens":1,"cached_tokens_by_modality":[{"modality":"image","tokens":1}]}`},
		{"inconsistent text breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":2}]}`},
		{"empty input breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"input_tokens_by_modality":[]}`},
		{"empty output breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"output_tokens_by_modality":[]}`},
		{"empty cache breakdown", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0,"total_cached_tokens":1,"cached_tokens_by_modality":[]}`},
		{"unpriced dimension", `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_other_tokens":1}`},
		{"over cap", `{"total_input_tokens":3,"total_output_tokens":120,"total_thought_tokens":9,"total_tokens":132}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"i","model":"gemini-3.8-flash","status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"ok"}]}],"usage":` + tc.usage + `}`))
			}))
			defer srv.Close()
			m := googleTextTestManager(t, srv.URL)
			route, _ := m.ResolveTextRoute(context.Background(), "text")
			resp, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
			if err == nil && resp != nil && resp.UsageKnown {
				t.Fatalf("invalid usage admitted: %+v", resp)
			}
		})
	}
}

func TestGoogleInteractionsIncompleteHasUsageButNoAction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"i","model":"gemini-3.8-flash","status":"incomplete","steps":[{"type":"model_output","content":[{"type":"text","text":"{\"Kind\":\"propose\"}"}]}],"usage":{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0}}`))
	}))
	defer srv.Close()
	m := googleTextTestManager(t, srv.URL)
	route, _ := m.ResolveTextRoute(context.Background(), "text")
	resp, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
	if err != nil || resp == nil || !resp.UsageKnown || len(resp.Choices) != 1 || resp.Choices[0].FinishReason == "stop" {
		t.Fatalf("response=%+v err=%v", resp, err)
	}
}

func TestGoogleInteractionsUnexpectedToolStepCannotBePriced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"i","model":"gemini-3.8-flash","status":"completed","steps":[{"type":"function_call","name":"unknown","arguments":{}},{"type":"model_output","content":[{"type":"text","text":"ok"}]}],"usage":{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0}}`))
	}))
	defer srv.Close()
	m := googleTextTestManager(t, srv.URL)
	route, _ := m.ResolveTextRoute(context.Background(), "text")
	response, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
	if err == nil && response != nil && response.UsageKnown {
		t.Fatalf("unexpected tool step was priced: %+v", response)
	}
}

func TestGoogleInteractionsDoesNotRetryOrRedirect(t *testing.T) {
	for _, status := range []int{307, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls, redirected atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
			defer target.Close()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":"sensitive-provider-body"}`))
			}))
			defer srv.Close()
			m := googleTextTestManager(t, srv.URL)
			route, _ := m.ResolveTextRoute(context.Background(), "text")
			_, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
			if !errors.Is(err, ErrTextOutcomeUnknown) || errors.Is(err, ErrTextNotDispatched) || calls.Load() != 1 || redirected.Load() != 0 || strings.Contains(err.Error(), "sensitive-provider-body") {
				t.Fatalf("calls=%d redirects=%d err=%v", calls.Load(), redirected.Load(), err)
			}
		})
	}
}

func TestGoogleInteractionsLostResponseDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()
	m := googleTextTestManager(t, srv.URL)
	route, _ := m.ResolveTextRoute(context.Background(), "text")
	_, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
	if !errors.Is(err, ErrTextOutcomeUnknown) || errors.Is(err, ErrTextNotDispatched) || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
}

func TestGoogleInteractionsUnknownKeepsSafeFailureReason(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		handle     http.HandlerFunc
	}{
		{"rate limited", "provider_http_429", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"sensitive-provider-body"}`))
		}},
		{"response malformed", "provider_response_json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not-json-sensitive-provider-body`))
		}},
		{"usage missing", "provider_usage_missing", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"i","model":"gemini-3.8-flash","status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"ok"}]}],"usage":{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14}}`))
		}},
		{"timeout", "provider_transport_timeout", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(1500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handle)
			defer srv.Close()
			m := googleTextTestManager(t, srv.URL)
			route, err := m.ResolveTextRoute(context.Background(), "text")
			if err != nil {
				t.Fatal(err)
			}
			response, err := m.CompleteText(context.Background(), "text", route, googleTextTestRequest())
			if err != nil && !errors.Is(err, ErrTextOutcomeUnknown) || err == nil && (response == nil || response.UsageKnown) {
				t.Fatalf("unknown outcome admitted: response=%+v err=%v", response, err)
			}
			got := TextOutcomeDiagnosticCode(err)
			if response != nil {
				got = response.OutcomeDiagnostic
			}
			if got != tc.want || strings.Contains(got, "sensitive") || strings.Contains(strings.ToLower(errString(err)), "sensitive") {
				t.Fatalf("diagnostic=%q err=%v, want %q", got, err, tc.want)
			}
		})
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestGoogleTransportHeaderTimeoutHasSafeTimeoutReason(t *testing.T) {
	err := &url.Error{Op: "Post", URL: "https://sensitive.example/path", Err: &net.DNSError{IsTimeout: true}}
	if got := googleTransportDiagnostic(err, context.Background()); got != "provider_transport_timeout" {
		t.Fatalf("header timeout classified as %q", got)
	}
}
