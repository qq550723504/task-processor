package einomodel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPinnedEinoComponentsUseOneGuardedSyntheticTransport(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     AdapterKind
		model    string
		response string
	}{
		{
			name: "openai-compatible", kind: AdapterOpenAICompatible, model: "synthetic-openai",
			response: "{\"id\":\"chatcmpl-test\",\"object\":\"chat.completion\",\"created\":1,\"model\":\"synthetic-openai\",\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}",
		},
		{
			name: "claude-native", kind: AdapterClaudeNative, model: "synthetic-claude",
			response: "{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"hello\"}],\"model\":\"synthetic-claude\",\"stop_reason\":\"end_turn\",\"stop_sequence\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sends atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				require.Equal(t, "POST", r.Method)
				require.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			guard, err := NewGuardedClient(GuardConfig{
				Adapter: tc.kind, Endpoint: server.URL, ModelID: tc.model,
				MaximumOutputTokens: 32, MaximumRequestBytes: 128 << 10,
				MaximumResponseBytes: 256 << 10, Timeout: 5 * time.Second,
			}, nil, func(context.Context) error { return nil })
			require.NoError(t, err)
			component, err := NewComponent(context.Background(), ComponentConfig{
				Adapter: tc.kind, Endpoint: server.URL, APIKey: "synthetic-only",
				ModelID: tc.model, MaximumOutputTokens: 32,
			}, guard)
			require.NoError(t, err)
			output, err := GenerateText(context.Background(), component, "system", "user", guard)
			if err != nil {
				t.Logf("synthetic output=%+v usage=%+v sends=%d", output, guard.Usage(), guard.NetworkSends())
			}
			require.NoError(t, err)
			require.Equal(t, "hello", output.Content)
			require.True(t, output.Usage.Known)
			require.Equal(t, 10, output.Usage.PromptTokens)
			require.Equal(t, 5, output.Usage.CompletionTokens)
			require.EqualValues(t, 1, sends.Load())
			require.EqualValues(t, 1, guard.NetworkSends())
		})
	}
}

func TestPinnedComponentsCannotRetryOrRedirectAfterHandoff(t *testing.T) {
	for _, tc := range []struct {
		kind  AdapterKind
		model string
	}{
		{AdapterOpenAICompatible, "synthetic-openai"},
		{AdapterClaudeNative, "synthetic-claude"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			var sends atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				w.Header().Set("Retry-After", "0")
				http.Error(w, "try again", http.StatusTooManyRequests)
			}))
			defer server.Close()
			guard, err := NewGuardedClient(GuardConfig{
				Adapter: tc.kind, Endpoint: server.URL, ModelID: tc.model,
				MaximumOutputTokens: 32, MaximumRequestBytes: 128 << 10,
				MaximumResponseBytes: 256 << 10, Timeout: 2 * time.Second,
			}, nil, func(context.Context) error { return nil })
			require.NoError(t, err)
			component, err := NewComponent(context.Background(), ComponentConfig{
				Adapter: tc.kind, Endpoint: server.URL, APIKey: "synthetic-only",
				ModelID: tc.model, MaximumOutputTokens: 32,
			}, guard)
			require.NoError(t, err)
			_, err = GenerateText(context.Background(), component, "system", "user", guard)
			require.Error(t, err)
			require.EqualValues(t, 1, sends.Load())
			require.EqualValues(t, 1, guard.NetworkSends())
		})
	}
}
