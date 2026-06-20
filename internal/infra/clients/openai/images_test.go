package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientGenerateImageUsesOpenAICompatibleEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var req ImageGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "nanobanana" {
			t.Fatalf("model = %q", req.Model)
		}
		w.Header().Set("X-Request-Id", "req-openai-1")
		_ = json.NewEncoder(w).Encode(ImageResponse{
			Usage: Usage{TotalTokens: 321},
			Data:  []ImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("pngdata"))}},
		})
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		APIKey:     "test-key",
		Model:      "nanobanana",
		BaseURL:    server.URL,
		Timeout:    time.Second,
		MaxRetries: 0,
	})
	resp, err := client.GenerateImage(context.Background(), &ImageGenerateRequest{
		Prompt: "generate scene",
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].B64JSON == "" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.RequestID != "req-openai-1" {
		t.Fatalf("request id = %q, want req-openai-1", resp.RequestID)
	}
	if resp.Usage.TotalTokens != 321 {
		t.Fatalf("usage = %+v, want total_tokens=321", resp.Usage)
	}
	if !strings.Contains(resp.RawResponse, "\"b64_json\"") {
		t.Fatalf("raw response = %q, want encoded image payload", resp.RawResponse)
	}
}

func TestClientEditImageUsesOpenAICompatibleEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		mediaType := r.Header.Get("Content-Type")
		if !strings.Contains(mediaType, "multipart/form-data") {
			t.Fatalf("content-type = %q", mediaType)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		var sawPrompt bool
		var sawImage bool
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "prompt":
				sawPrompt = string(data) == "edit faithfully"
			case "image":
				sawImage = len(data) > 0
			}
		}
		if !sawPrompt || !sawImage {
			t.Fatalf("multipart request missing expected fields")
		}
		_ = json.NewEncoder(w).Encode(ImageResponse{
			Data: []ImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("edited"))}},
		})
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		APIKey:     "test-key",
		Model:      "nanobanana",
		BaseURL:    server.URL,
		Timeout:    time.Second,
		MaxRetries: 0,
	})
	resp, err := client.EditImage(context.Background(), &ImageEditRequest{
		Prompt: "edit faithfully",
		Image:  []byte("source"),
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].B64JSON == "" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestBuildAPIURL(t *testing.T) {
	got := buildAPIURL("https://example.com/v1/", "/images/generations")
	if got != "https://example.com/v1/images/generations" {
		t.Fatalf("buildAPIURL() = %q", got)
	}
}

func TestClientDoesNotSupportAsyncImageGenerationByDefault(t *testing.T) {
	client := NewClient(&ClientConfig{
		APIKey:     "test-key",
		Model:      "gpt-image-2",
		BaseURL:    "https://example.invalid",
		Timeout:    time.Second,
		MaxRetries: 0,
	})

	if client.SupportsAsyncImageGeneration() {
		t.Fatal("SupportsAsyncImageGeneration() = true, want false")
	}

	_, err := client.SubmitImageGeneration(context.Background(), &ImageGenerateRequest{
		Prompt: "flat artwork",
	})
	if !errors.Is(err, ErrAsyncImageGenerationNotSupported) {
		t.Fatalf("SubmitImageGeneration() error = %v, want ErrAsyncImageGenerationNotSupported", err)
	}
}
