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

func TestClientEditImageUsesSubmitPollFlow(t *testing.T) {
	var pollCount int32
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
			var req submitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode submit request: %v", err)
			}
			if req.Model != "nano-banana-fast" {
				t.Fatalf("model = %q", req.Model)
			}
			if len(req.Images) != 2 || req.Images[0] != "https://example.com/source.png" || req.Images[1] != "https://example.com/side.png" {
				t.Fatalf("images = %#v", req.Images)
			}
			if req.ReplyType != "async" {
				t.Fatalf("replyType = %q", req.ReplyType)
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "running",
			})
		case "/v1/api/result":
			if r.Method != http.MethodGet {
				t.Fatalf("poll method = %s", r.Method)
			}
			if got := r.URL.Query().Get("id"); got != "job-1" {
				t.Fatalf("query id = %q", got)
			}
			current := atomic.AddInt32(&pollCount, 1)
			if current == 1 {
				_ = json.NewEncoder(w).Encode(resultPayload{
					ID:       "job-1",
					Status:   "running",
					Progress: 40,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(resultPayload{
				ID:       "job-1",
				Status:   "succeeded",
				Progress: 100,
				Results: []resultItem{
					{URL: serverURL + "/generated.png", Content: "done"},
				},
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
	if resp.Data[0].URL == "" {
		t.Fatal("expected result url")
	}
	wantB64 := base64.StdEncoding.EncodeToString(imageBytes)
	if resp.Data[0].B64JSON != wantB64 {
		t.Fatalf("b64_json = %q, want %q", resp.Data[0].B64JSON, wantB64)
	}
	if atomic.LoadInt32(&pollCount) < 2 {
		t.Fatalf("expected poll flow, got %d polls", pollCount)
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

func TestBuildSubmitURLUsesCompletionsEndpointForGPTImage(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{
			name: "nano endpoint",
			base: "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			want: "https://grsai.dakka.com.cn/v1/draw/completions",
		},
		{
			name: "host only",
			base: "https://grsai.dakka.com.cn",
			want: "https://grsai.dakka.com.cn/v1/draw/completions",
		},
		{
			name: "already completions",
			base: "https://grsai.dakka.com.cn/v1/draw/completions",
			want: "https://grsai.dakka.com.cn/v1/draw/completions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSubmitURL(tt.base, "gpt-image-2")
			if err != nil {
				t.Fatalf("buildSubmitURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildSubmitURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSubmitURLUsesGenerateEndpointForNanoBananaModels(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{
			name: "v1 base",
			base: "https://grsaiapi.com/v1",
			want: "https://grsaiapi.com/v1/api/generate",
		},
		{
			name: "legacy draw path",
			base: "https://grsai.dakka.com.cn/v1/draw/nano-banana",
			want: "https://grsai.dakka.com.cn/v1/api/generate",
		},
		{
			name: "already generate path",
			base: "https://grsaiapi.com/v1/api/generate",
			want: "https://grsaiapi.com/v1/api/generate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSubmitURL(tt.base, "nano-banana-fast")
			if err != nil {
				t.Fatalf("buildSubmitURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildSubmitURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientEditImageReturnsTypedModerationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "job-1",
				Status: "running",
			})
		case "/v1/api/result":
			_ = json.NewEncoder(w).Encode(resultPayload{
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

func TestClientEditImageTimesOutStuckRunningJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     "019db4c4-6d2e-7592-9978-723fc89ef5e9",
				Status: "running",
			})
		case "/v1/api/result":
			_ = json.NewEncoder(w).Encode(resultPayload{
				ID:       "019db4c4-6d2e-7592-9978-723fc89ef5e9",
				Status:   "running",
				Progress: 80,
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
		Timeout:      50 * time.Millisecond,
		HTTPClient:   server.Client(),
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
			Prompt:   "edit faithfully",
			ImageURL: "https://example.com/source.png",
		})
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !strings.Contains(err.Error(), "019db4c4-6d2e-7592-9978-723fc89ef5e9") {
			t.Fatalf("error = %q, want session id", err)
		}
		if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
			t.Fatalf("error = %q, want deadline exceeded", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("EditImage() did not stop polling a stuck running job")
	}
}

func TestClientGenerateImageRetriesTransientJobFailure(t *testing.T) {
	var submitCount int32
	var resultCount int32
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			id := "job-1"
			if atomic.AddInt32(&submitCount, 1) > 1 {
				id = "job-2"
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				ID:     id,
				Status: "running",
			})
		case "/v1/api/result":
			current := atomic.AddInt32(&resultCount, 1)
			if current == 1 {
				_ = json.NewEncoder(w).Encode(resultPayload{
					ID:     "job-1",
					Status: "failed",
					Error:  "google gemini timeout...",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(resultPayload{
				ID:       "job-2",
				Status:   "succeeded",
				Progress: 100,
				Results: []resultItem{
					{URL: serverURL + "/generated.png", Content: "done"},
				},
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
				Status: "running",
			})
		case "/v1/api/result":
			_ = json.NewEncoder(w).Encode(resultPayload{
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

func TestClientGenerateImageUsesNewGenerateAPIForV1BaseURL(t *testing.T) {
	var submitCount int32
	var pollCount int32
	imageBytes := []byte("generated-image")
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			current := atomic.AddInt32(&submitCount, 1)
			if current == 1 {
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
				if req["replyType"] != "async" {
					t.Fatalf("replyType = %#v", req["replyType"])
				}
				if req["prompt"] != "flat pod artwork" {
					t.Fatalf("prompt = %#v", req["prompt"])
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":     "job-1",
					"status": "running",
				})
				return
			}

		case "/v1/api/result":
			if got := r.URL.Query().Get("id"); got != "job-1" {
				t.Fatalf("query id = %q", got)
			}
			currentPoll := atomic.AddInt32(&pollCount, 1)
			if currentPoll == 1 {
				_ = json.NewEncoder(w).Encode(resultPayload{
					ID:       "job-1",
					Status:   "running",
					Progress: 50,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(resultPayload{
				ID:       "job-1",
				Status:   "succeeded",
				Progress: 100,
				Results: []resultItem{
					{URL: serverURL + "/generated.png"},
				},
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
	if atomic.LoadInt32(&submitCount) != 1 {
		t.Fatalf("submit count = %d, want 1", submitCount)
	}
	if atomic.LoadInt32(&pollCount) != 2 {
		t.Fatalf("poll count = %d, want 2", pollCount)
	}
}
