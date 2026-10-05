package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/integration/googleinteractions"
)

type googleTitleFixtureTransport func(*http.Request) (*http.Response, error)

func (f googleTitleFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func isolatedGoogleTitleTransport(t *testing.T, endpoint string) http.RoundTripper {
	t.Helper()
	target, err := url.Parse(endpoint)
	require.NoError(t, err)
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DisableKeepAlives = true
	t.Cleanup(base.CloseIdleConnections)
	return googleTitleFixtureTransport(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, googleinteractions.Origin, r.URL.Scheme+"://"+r.URL.Host)
		require.Equal(t, "/v1beta/interactions", r.URL.Path)
		forward := r.Clone(r.Context())
		forward.URL.Scheme, forward.URL.Host, forward.Host = target.Scheme, target.Host, target.Host
		return base.RoundTrip(forward)
	})
}

func googleTitleFixtureReply(t *testing.T, w http.ResponseWriter, content string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"id": "interaction-fixture", "model": googleinteractions.Model, "status": "completed",
		"steps": []any{map[string]any{"type": "thought", "content": []any{}}, map[string]any{"type": "model_output", "content": []any{map[string]any{"type": "text", "text": content}}}},
		"usage": map[string]int{"total_input_tokens": 20, "total_output_tokens": 10, "total_thought_tokens": 4, "total_tokens": 34, "total_tool_use_tokens": 0},
	}))
}

func TestProductAgentGoogleUsesSharedExecutorWithKnowledge(t *testing.T) {
	testProductAgentOwners(t, "google knowledge")
}
