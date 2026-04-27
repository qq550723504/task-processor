package nanobanana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
		case "/v1/draw/nano-banana":
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
			if len(req.URLs) != 2 || req.URLs[0] != "https://example.com/source.png" || req.URLs[1] != "https://example.com/side.png" {
				t.Fatalf("urls = %#v", req.URLs)
			}
			if req.WebHook != "-1" {
				t.Fatalf("webHook = %q", req.WebHook)
			}
			_ = json.NewEncoder(w).Encode(submitResponse{
				Code: 0,
				Msg:  "success",
				Data: struct {
					ID string `json:"id"`
				}{ID: "job-1"},
			})
		case "/v1/draw/result":
			if r.Method != http.MethodPost {
				t.Fatalf("poll method = %s", r.Method)
			}
			current := atomic.AddInt32(&pollCount, 1)
			if current == 1 {
				_ = json.NewEncoder(w).Encode(resultEnvelope{
					Code: 0,
					Msg:  "success",
					Data: resultPayload{
						ID:       "job-1",
						Status:   "running",
						Progress: 40,
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(resultEnvelope{
				Code: 0,
				Msg:  "success",
				Data: resultPayload{
					ID:       "job-1",
					Status:   "succeeded",
					Progress: 100,
					Results: []resultItem{
						{URL: serverURL + "/generated.png", Content: "done"},
					},
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
		SubmitURL:    server.URL + "/v1/draw/nano-banana",
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
		SubmitURL:    "https://example.com/v1/draw/nano-banana",
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

func TestClientEditImageReturnsTypedModerationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/draw/nano-banana":
			_ = json.NewEncoder(w).Encode(submitResponse{
				Code: 0,
				Msg:  "success",
				Data: struct {
					ID string `json:"id"`
				}{ID: "job-1"},
			})
		case "/v1/draw/result":
			_ = json.NewEncoder(w).Encode(resultEnvelope{
				Code: 0,
				Msg:  "success",
				Data: resultPayload{
					ID:            "job-1",
					Status:        "failed",
					FailureReason: "output_moderation",
					Error:         "blocked by provider moderation",
				},
			})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:       "test-key",
		Model:        "nano-banana-fast",
		SubmitURL:    server.URL + "/v1/draw/nano-banana",
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
	if jobErr.FailureReason() != "output_moderation" {
		t.Fatalf("failure reason = %q", jobErr.FailureReason())
	}
}
