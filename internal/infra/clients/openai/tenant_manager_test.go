package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type capturedOpenAIRequest struct {
	Auth  string
	Model string
}

func newCaptureChatServer(t *testing.T, requests *[]capturedOpenAIRequest, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		*requests = append(*requests, capturedOpenAIRequest{
			Auth:  r.Header.Get("Authorization"),
			Model: payload.Model,
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   payload.Model,
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": "ok",
				},
				"finish_reason": "stop",
			}},
		})
	}))
}

func TestManagerRoutesChatClientByTenantOrUserContext(t *testing.T) {
	var mu sync.Mutex
	var defaultRequests []capturedOpenAIRequest
	var tenantRequests []capturedOpenAIRequest
	var userRequests []capturedOpenAIRequest
	defaultServer := newCaptureChatServer(t, &defaultRequests, &mu)
	defer defaultServer.Close()
	tenantServer := newCaptureChatServer(t, &tenantRequests, &mu)
	defer tenantServer.Close()
	userServer := newCaptureChatServer(t, &userRequests, &mu)
	defer userServer.Close()

	mgr, err := NewManager(&ManagerConfig{
		Clients: map[string]*ClientConfig{
			"default": testClientConfig("default-key", "default-model", defaultServer.URL),
		},
		DefaultClient: "default",
		ConfigResolver: fakeClientConfigResolver{
			tenantConfigs: map[string]*ClientConfig{
				"tenant-a": testClientConfig("tenant-key", "tenant-model", tenantServer.URL),
			},
			userConfigs: map[string]*ClientConfig{
				"user-a": testClientConfig("user-key", "user-model", userServer.URL),
			},
		},
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	client, err := mgr.GetClient("default")
	if err != nil {
		t.Fatalf("GetClient() error = %v", err)
	}

	callChat(t, client, context.Background())
	callChat(t, client, WithTenantID(context.Background(), "tenant-a"))
	callChat(t, client, WithIdentity(context.Background(), Identity{TenantID: "tenant-a", UserID: "user-a"}))

	if len(defaultRequests) != 1 || defaultRequests[0].Auth != "Bearer default-key" || defaultRequests[0].Model != "default-model" {
		t.Fatalf("default requests = %#v", defaultRequests)
	}
	if len(tenantRequests) != 1 || tenantRequests[0].Auth != "Bearer tenant-key" || tenantRequests[0].Model != "tenant-model" {
		t.Fatalf("tenant requests = %#v", tenantRequests)
	}
	if len(userRequests) != 1 || userRequests[0].Auth != "Bearer user-key" || userRequests[0].Model != "user-model" {
		t.Fatalf("user requests = %#v", userRequests)
	}
}

type fakeClientConfigResolver struct {
	tenantConfigs map[string]*ClientConfig
	userConfigs   map[string]*ClientConfig
}

func (r fakeClientConfigResolver) ResolveClientConfig(ctx context.Context, clientName string, fallback *ClientConfig) (*ResolvedClientConfig, error) {
	identity := IdentityFromContext(ctx)
	if identity.UserID != "" {
		if cfg := r.userConfigs[identity.UserID]; cfg != nil {
			return &ResolvedClientConfig{CacheKey: "user:" + identity.UserID + ":" + clientName, Config: cfg}, nil
		}
	}
	if identity.TenantID != "" {
		if cfg := r.tenantConfigs[identity.TenantID]; cfg != nil {
			return &ResolvedClientConfig{CacheKey: "tenant:" + identity.TenantID + ":" + clientName, Config: cfg}, nil
		}
	}
	return nil, nil
}

func testClientConfig(apiKey, model, baseURL string) *ClientConfig {
	return &ClientConfig{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		Timeout:    time.Second,
		MaxRetries: 0,
		RetryDelay: time.Millisecond,
	}
}

func callChat(t *testing.T, client ChatCompleter, ctx context.Context) {
	t.Helper()
	_, err := client.CreateChatCompletion(ctx, &ChatCompletionRequest{
		Messages: []ChatCompletionMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion() error = %v", err)
	}
}
