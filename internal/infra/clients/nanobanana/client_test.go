package nanobanana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestClientEditImageUsesGenerateEndpointForNanoBanana(t *testing.T) {
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req["model"] != "nano-banana-fast" {
				t.Fatalf("model = %#v", req["model"])
			}
			if req["prompt"] != "edit faithfully" {
				t.Fatalf("prompt = %#v", req["prompt"])
			}
			if req["response_format"] != "url" {
				t.Fatalf("response_format = %#v", req["response_format"])
			}
			images, ok := req["image"].([]any)
			if !ok || len(images) != 2 {
				t.Fatalf("image = %#v", req["image"])
			}
			if images[0] != "https://example.com/source.png" || images[1] != "https://example.com/side.png" {
				t.Fatalf("image urls = %#v", images)
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "succeeded",
				Results: []resultItem{{
					URL: serverURL + "/generated.png",
				}},
			})
		case "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		HTTPClient:   server.Client(),
	})

	resp, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt: "edit faithfully",
		ImageURLs: []string{
			" https://example.com/source.png ",
			"https://example.com/source.png",
			"https://example.com/side.png",
		},
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if resp.Data[0].URL != serverURL+"/generated.png" {
		t.Fatalf("url = %q", resp.Data[0].URL)
	}
	wantB64 := base64.StdEncoding.EncodeToString(imageBytes)
	if resp.Data[0].B64JSON != wantB64 {
		t.Fatalf("b64_json = %q, want %q", resp.Data[0].B64JSON, wantB64)
	}
}

func TestClientEditImageRequiresImageURL(t *testing.T) {
	client := NewClient(Config{
		Model:        "nano-banana-fast",
		SubmitURL:    "https://example.com/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
	})

	_, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt: "edit faithfully",
		Image:  []byte("raw-bytes-only"),
	})
	if err == nil {
		t.Fatal("expected error for missing image url")
	}
}

func TestBuildSubmitURLUsesGenerateEndpoint(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		model string
		want  string
	}{
		{
			name:  "nano model on v1 base",
			base:  "https://grsaiapi.com/v1",
			model: "nano-banana-fast",
			want:  "https://grsaiapi.com/v1/api/generate",
		},
		{
			name:  "gpt model on host only",
			base:  "https://grsai.dakka.com.cn",
			model: "gpt-image-2",
			want:  "https://grsai.dakka.com.cn/v1/api/generate",
		},
		{
			name:  "legacy draw path",
			base:  "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			model: "nano-banana-fast",
			want:  "https://grsai.dakka.com.cn/v1/api/generate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSubmitURL(tt.base, tt.model)
			if err != nil {
				t.Fatalf("buildSubmitURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildSubmitURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientEditImageUsesGenerateEndpointForGPTImage(t *testing.T) {
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req["model"] != "gpt-image-2" {
				t.Fatalf("model = %#v", req["model"])
			}
			if req["prompt"] != "edit faithfully" {
				t.Fatalf("prompt = %#v", req["prompt"])
			}
			if req["size"] != "1024x1024" {
				t.Fatalf("size = %#v", req["size"])
			}
			if req["response_format"] != "url" {
				t.Fatalf("response_format = %#v", req["response_format"])
			}
			images, ok := req["image"].([]any)
			if !ok || len(images) != 2 {
				t.Fatalf("image = %#v", req["image"])
			}
			if images[0] != "https://example.com/source.png" || images[1] != "https://example.com/side.png" {
				t.Fatalf("image urls = %#v", images)
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "succeeded",
				Results: []resultItem{{
					URL: serverURL + "/generated.png",
				}},
			})
		case "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "gpt-image-2",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		HTTPClient:   server.Client(),
	})

	resp, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Model:  "gpt-image-2",
		Prompt: "edit faithfully",
		ImageURLs: []string{
			"https://example.com/source.png",
			"https://example.com/side.png",
		},
		Size: "1024x1024",
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if resp.Data[0].URL != serverURL+"/generated.png" {
		t.Fatalf("url = %q", resp.Data[0].URL)
	}
	if resp.Data[0].B64JSON == "" {
		t.Fatal("expected b64_json")
	}
}

func TestClientSubmitImageGenerationReturnsAsyncJobMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/generate" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("X-Request-Id", "req-submit-1")
		_ = json.NewEncoder(w).Encode(submitResponse{
			ID:     "job-async-1",
			Status: "running",
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		Model:      "nano-banana-fast",
		SubmitURL:  server.URL + "/v1",
		Timeout:    time.Second,
		HTTPClient: server.Client(),
	})

	result, err := client.SubmitImageGeneration(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "flat artwork",
	})
	if err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if result.JobID != "job-async-1" {
		t.Fatalf("job id = %q, want job-async-1", result.JobID)
	}
	if result.RequestID != "req-submit-1" {
		t.Fatalf("request id = %q, want req-submit-1", result.RequestID)
	}
	if result.Provider != "nanobanana" {
		t.Fatalf("provider = %q, want nanobanana", result.Provider)
	}
}

func TestClientGenerateImagePollsAsyncResultUntilSucceeded(t *testing.T) {
	imageBytes := []byte("generated-image")
	var serverURL string
	var resultCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/api/generate":
			w.Header().Set("X-Request-Id", "req-nb-submit")
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-async",
				Status: "running",
			})
		case r.URL.Path == "/v1/api/result":
			call := atomic.AddInt32(&resultCalls, 1)
			if got := r.URL.Query().Get("id"); got != "job-async" {
				t.Fatalf("query id = %q", got)
			}
			status := "running"
			var results []resultItem
			if call >= 2 {
				status = "succeeded"
				results = []resultItem{{URL: serverURL + "/generated.png"}}
			}
			_ = json.NewEncoder(w).Encode(resultPayload{
				ID:       "job-async",
				Status:   status,
				Results:  results,
				Progress: int(call) * 50,
			})
		case r.URL.Path == "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "gpt-image-2",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		MaxAttempts:  3,
		HTTPClient:   server.Client(),
	})

	resp, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "flat pod artwork",
		Size:   "1024x1024",
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if resultCalls < 2 {
		t.Fatalf("result calls = %d, want at least 2", resultCalls)
	}
	if len(resp.Data) != 1 || resp.Data[0].URL != serverURL+"/generated.png" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.RequestID != "req-nb-submit" {
		t.Fatalf("request id = %q, want req-nb-submit", resp.RequestID)
	}
	if resp.UpstreamJobID != "job-async" {
		t.Fatalf("upstream job id = %q, want job-async", resp.UpstreamJobID)
	}
	if !strings.Contains(resp.RawResponse, "\"status\":\"succeeded\"") {
		t.Fatalf("raw response = %q, want final polled payload", resp.RawResponse)
	}
}

func TestClientQueryImageGenerationReturnsSucceededResultPayload(t *testing.T) {
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/result":
			if got := r.URL.Query().Get("id"); got != "job-query-1" {
				t.Fatalf("query id = %q", got)
			}
			w.Header().Set("X-Request-Id", "req-query-1")
			_ = json.NewEncoder(w).Encode(resultPayload{
				ID:      "job-query-1",
				Status:  "succeeded",
				Results: []resultItem{{URL: serverURL + "/generated.png"}},
			})
		case "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		HTTPClient:   server.Client(),
	})

	result, err := client.QueryImageGeneration(context.Background(), "job-query-1")
	if err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if result.RequestID != "req-query-1" {
		t.Fatalf("request id = %q, want req-query-1", result.RequestID)
	}
	if len(result.Data) != 1 || result.Data[0].URL != serverURL+"/generated.png" {
		t.Fatalf("result data = %+v, want downloaded image metadata", result.Data)
	}
}

func TestClientEditImageReturnsTypedModerationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "violation",
				Error:  "blocked by provider moderation",
			})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		HTTPClient:   server.Client(),
	})

	_, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt:   "edit faithfully",
		ImageURL: "https://example.com/source.png",
	})
	if err == nil {
		t.Fatal("expected moderation error")
	}
	var jobErr *JobError
	if !errors.As(err, &jobErr) {
		t.Fatalf("error = %T, want *JobError", err)
	}
	if jobErr.FailureReason() != "violation" {
		t.Fatalf("failure reason = %q", jobErr.FailureReason())
	}
}

func TestClientGenerateImageRetriesTransientHTTPFailure(t *testing.T) {
	var submitCount int32
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			if atomic.AddInt32(&submitCount, 1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
				return
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "succeeded",
				Results: []resultItem{{
					URL: serverURL + "/generated.png",
				}},
			})
		case "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		MaxAttempts:  2,
		HTTPClient:   server.Client(),
	})

	resp, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "flat pod artwork",
		Size:   "1024x1024",
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if atomic.LoadInt32(&submitCount) != 2 {
		t.Fatalf("submit count = %d, want 2", submitCount)
	}
}

func TestClientGenerateImageDoesNotRetryModerationFailure(t *testing.T) {
	var submitCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			atomic.AddInt32(&submitCount, 1)
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "violation",
				Error:  "blocked by provider moderation",
			})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		MaxAttempts:  3,
		HTTPClient:   server.Client(),
	})

	_, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "flat pod artwork",
		Size:   "1024x1024",
	})
	if err == nil {
		t.Fatal("expected moderation error")
	}
	if atomic.LoadInt32(&submitCount) != 1 {
		t.Fatalf("submit count = %d, want 1", submitCount)
	}
}

func TestClientGenerateImageUsesGenerateEndpointForNanoBanana(t *testing.T) {
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			if r.Method != http.MethodPost {
				t.Fatalf("submit method = %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode submit request: %v", err)
			}
			if req["model"] != "nano-banana-fast" {
				t.Fatalf("model = %#v", req["model"])
			}
			if req["prompt"] != "flat pod artwork" {
				t.Fatalf("prompt = %#v", req["prompt"])
			}
			if req["size"] != "1024x1024" {
				t.Fatalf("size = %#v", req["size"])
			}
			if req["response_format"] != "url" {
				t.Fatalf("response_format = %#v", req["response_format"])
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "succeeded",
				Results: []resultItem{{
					URL: serverURL + "/generated.png",
				}},
			})
		case "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1",
		PollInterval: 10 * time.Millisecond,
		Timeout:      time.Second,
		HTTPClient:   server.Client(),
	})

	resp, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "flat pod artwork",
		Size:   "1024x1024",
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if resp.Data[0].URL != serverURL+"/generated.png" {
		t.Fatalf("url = %q", resp.Data[0].URL)
	}
}
