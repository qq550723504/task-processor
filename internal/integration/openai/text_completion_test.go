package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func textTestManager(t *testing.T, base string) *Manager {
	t.Helper()
	m, err := NewManager(&ManagerConfig{Clients: map[string]*ClientConfig{"text": {
		APIKey: "test-only", Model: "gemini-2.5-flash", APIStyle: "grsai", BaseURL: base + "/v1", Timeout: time.Second, MaxRetries: 3,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func textTestRequest() TextCompletionRequest {
	return TextCompletionRequest{System: "Return JSON.", Prompt: "diagnose this product", MaximumOutputTokens: 128}
}

func TestTextCompletionGRSAIWireAndUsage(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-only" || payload["model"] != "gemini-2.5-flash" || payload["max_tokens"] != float64(128) || payload["stream"] == true {
			t.Errorf("unexpected request: %s %s %#v", r.Method, r.URL.Path, payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"provider-1","model":"gemini-2.5-flash","choices":[{"message":{"role":"assistant","content":"{\"Kind\":\"interrupt\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":261,"total_tokens":263}}`))
	}))
	defer srv.Close()
	m := textTestManager(t, srv.URL)
	route, err := m.ResolveTextRoute(context.Background(), "text")
	if err != nil || route.ProviderID != "grsai" {
		t.Fatalf("route=%+v err=%v", route, err)
	}
	resp, err := m.CompleteText(context.Background(), "text", route, textTestRequest())
	if err != nil || resp == nil || !resp.UsageKnown || resp.ID != "provider-1" || resp.Usage.TotalTokens != 263 || resp.Choices[0].Message.Content != `{"Kind":"interrupt"}` || calls.Load() != 1 {
		t.Fatalf("response=%+v err=%v calls=%d", resp, err, calls.Load())
	}
}

func TestTextCompletionUsageRequiresEveryProviderCounter(t *testing.T) {
	for _, tc := range []struct {
		name, usage string
		known       bool
	}{
		{"absent", ``, false},
		{"null", `,"usage":null`, false},
		{"missing completion", `,"usage":{"prompt_tokens":3,"total_tokens":3}`, false},
		{"missing prompt", `,"usage":{"completion_tokens":3,"total_tokens":3}`, false},
		{"null prompt", `,"usage":{"prompt_tokens":null,"completion_tokens":3,"total_tokens":3}`, false},
		{"inconsistent", `,"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":9}`, false},
		{"negative", `,"usage":{"prompt_tokens":-1,"completion_tokens":4,"total_tokens":3}`, false},
		{"explicit zero completion", `,"usage":{"prompt_tokens":3,"completion_tokens":0,"total_tokens":3}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]` + tc.usage + `}`))
			}))
			defer srv.Close()
			m := textTestManager(t, srv.URL)
			route, _ := m.ResolveTextRoute(context.Background(), "text")
			resp, err := m.CompleteText(context.Background(), "text", route, textTestRequest())
			if err != nil || resp.UsageKnown != tc.known {
				t.Fatalf("known=%v response=%+v err=%v", tc.known, resp, err)
			}
		})
	}
}

func TestTextRouteRecognizesExistingGRSAIConfiguration(t *testing.T) {
	for _, base := range []string{"https://grsaiapi.com/v1", "https://grsai.dakka.com.cn/v1"} {
		m, err := NewManager(&ManagerConfig{Clients: map[string]*ClientConfig{"default": NewClientConfig("test-only", "gemini-2.5-flash", base, 30)}})
		requireNoErrorText(t, err)
		route, err := m.ResolveTextRoute(context.Background(), "default")
		requireNoErrorText(t, err)
		if route.ProviderID != "grsai" {
			t.Fatalf("misattributed provider: %+v", route)
		}
		_ = m.Close()
	}
}

func requireNoErrorText(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestTextCompletionConfiguredTimeoutIncludesQueue(t *testing.T) {
	m := textTestManager(t, "http://127.0.0.1:1")
	m.clients["text"].config.Timeout = 50 * time.Millisecond
	route, err := m.ResolveTextRoute(context.Background(), "text")
	requireNoErrorText(t, err)
	pool := m.clients["text"].pool
	for i := 0; i < cap(pool.semaphore); i++ {
		pool.semaphore <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(pool.semaphore); i++ {
			<-pool.semaphore
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := time.Now()
	_, err = m.CompleteText(ctx, "text", route, textTestRequest())
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 700*time.Millisecond {
		t.Fatalf("err=%v duration=%v", err, time.Since(started))
	}
}

func TestTextCompletionRejectsStaleRouteBeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	m := textTestManager(t, srv.URL)
	route, err := m.ResolveTextRoute(context.Background(), "text")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*EffectiveClientRoute){
		func(r *EffectiveClientRoute) { r.ModelID = "other" },
		func(r *EffectiveClientRoute) { r.ProviderID = "other" },
		func(r *EffectiveClientRoute) { r.CredentialReference = "other" },
		func(r *EffectiveClientRoute) { r.ConfigurationVersion = "stale" },
	} {
		stale := route
		mutate(&stale)
		if _, err := m.CompleteText(context.Background(), "text", stale, textTestRequest()); !errors.Is(err, ErrClientConfigurationChanged) {
			t.Fatalf("got %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("stale quote dispatched")
	}
}

func TestTextCompletionDoesNotRetryOrFollowRedirects(t *testing.T) {
	for _, status := range []int{307, 308, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls, redirected atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
			defer target.Close()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"message":"sensitive-provider-body","type":"error"}}`))
			}))
			defer srv.Close()
			m := textTestManager(t, srv.URL)
			route, _ := m.ResolveTextRoute(context.Background(), "text")
			_, err := m.CompleteText(context.Background(), "text", route, textTestRequest())
			if !errors.Is(err, ErrTextOutcomeUnknown) || calls.Load() != 1 || redirected.Load() != 0 || strings.Contains(err.Error(), "sensitive-provider-body") {
				t.Fatalf("calls=%d redirects=%d err=%v", calls.Load(), redirected.Load(), err)
			}
		})
	}
}

func TestTextCompletionBoundsAndMissingUsage(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		fail       bool
	}{
		{"oversize", strings.Repeat("x", MaxTextResponseBytes+1), true},
		{"invalid", "bad json", true},
		{"missing usage", `{"choices":[{"message":{"content":"ok"}}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer srv.Close()
			m := textTestManager(t, srv.URL)
			route, _ := m.ResolveTextRoute(context.Background(), "text")
			resp, err := m.CompleteText(context.Background(), "text", route, textTestRequest())
			if tc.fail && !errors.Is(err, ErrTextOutcomeUnknown) {
				t.Fatalf("got %v", err)
			}
			if !tc.fail && (err != nil || resp.Usage.TotalTokens != 0) {
				t.Fatalf("invented usage: %+v %v", resp, err)
			}
		})
	}
}

func TestTextCompletionLostResponseIsSingleDispatch(t *testing.T) {
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
	m := textTestManager(t, srv.URL)
	route, _ := m.ResolveTextRoute(context.Background(), "text")
	_, err := m.CompleteText(context.Background(), "text", route, textTestRequest())
	if !errors.Is(err, ErrTextOutcomeUnknown) || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
}

func TestTextCompletionHonorsCallerDeadline(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()
	m := textTestManager(t, srv.URL)
	route, _ := m.ResolveTextRoute(context.Background(), "text")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := m.CompleteText(ctx, "text", route, textTestRequest())
	if !errors.Is(err, ErrTextOutcomeUnknown) || time.Since(started) > 700*time.Millisecond || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v duration=%v", calls.Load(), err, time.Since(started))
	}
	_, err = m.CompleteText(ctx, "text", route, textTestRequest())
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("cancelled input dispatched: %d %v", calls.Load(), err)
	}
}

type textChangingResolver struct {
	calls atomic.Int32
	base  string
}

func (r *textChangingResolver) ResolveClientConfig(_ context.Context, _ string, _ *ClientConfig) (*ResolvedClientConfig, error) {
	version := "one"
	if r.calls.Add(1) >= 3 {
		version = "two"
	}
	return &ResolvedClientConfig{CacheKey: version, Config: &ClientConfig{APIKey: "test-only", Model: "gemini-2.5-flash", APIStyle: "grsai", BaseURL: r.base + "/v1", Timeout: time.Second}}, nil
}

func TestTextCompletionChecksConfigurationAgainAfterQueue(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	m := textTestManager(t, srv.URL)
	m.SetConfigResolver(&textChangingResolver{base: srv.URL})
	route, err := m.ResolveTextRoute(context.Background(), "text")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.CompleteText(context.Background(), "text", route, textTestRequest())
	if !errors.Is(err, ErrClientConfigurationChanged) || calls.Load() != 0 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
}

func TestTextCompletionRejectsUnboundedInputBeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	m := textTestManager(t, srv.URL)
	route, _ := m.ResolveTextRoute(context.Background(), "text")
	for _, req := range []TextCompletionRequest{{System: "system", Prompt: "prompt"}, {System: "system", Prompt: strings.Repeat("x", MaxTextPromptBytes), MaximumOutputTokens: 32}} {
		if _, err := m.CompleteText(context.Background(), "text", route, req); !errors.Is(err, ErrTextInput) {
			t.Fatalf("got %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input dispatched")
	}
}
