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
		APIKey:                   "test-key",
		Model:                    "nanobanana",
		BaseURL:                  server.URL,
		Timeout:                  time.Second,
		MaxRetries:               0,
		ImageReferenceHTTPClient: server.Client(),
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
			case "image[]":
				sawImage = len(data) > 0 && part.FileName() == "image.webp"
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
		APIKey:                   "test-key",
		Model:                    "nanobanana",
		BaseURL:                  server.URL,
		Timeout:                  time.Second,
		MaxRetries:               0,
		ImageReferenceHTTPClient: server.Client(),
	})
	resp, err := client.EditImage(context.Background(), &ImageEditRequest{
		Prompt:           "edit faithfully",
		Image:            []byte("source"),
		ImageContentType: "image/webp",
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].B64JSON == "" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestClientEditImageIncludesSecondaryURLsAsMultipartImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/secondary.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("secondary-image"))
			return
		case "/images/edits":
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader: %v", err)
			}
			imageParts := 0
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("NextPart: %v", err)
				}
				data, _ := io.ReadAll(part)
				if part.FormName() == "image[]" {
					imageParts++
					if len(data) == 0 {
						t.Fatal("empty image part")
					}
				}
			}
			if imageParts != 2 {
				t.Fatalf("image parts = %d, want primary and secondary", imageParts)
			}
			_ = json.NewEncoder(w).Encode(ImageResponse{Data: []ImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("edited"))}}})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		APIKey:                   "test-key",
		Model:                    "gpt-image-1",
		BaseURL:                  server.URL,
		Timeout:                  time.Second,
		MaxRetries:               0,
		ImageReferenceHTTPClient: server.Client(),
	})
	if _, err := client.EditImage(context.Background(), &ImageEditRequest{
		Image:            []byte("primary-image"),
		ImageContentType: "image/png",
		ImageURLs:        []string{server.URL + "/secondary.png"},
	}); err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
}

func TestClientEditImageBoundsSecondaryReferenceDownloadByClientTimeout(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/secondary.png":
			close(started)
			<-r.Context().Done()
		case "/images/edits":
			t.Fatal("edit endpoint reached after secondary download timeout")
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		APIKey:                   "test-key",
		Model:                    "gpt-image-1",
		BaseURL:                  server.URL,
		Timeout:                  25 * time.Millisecond,
		MaxRetries:               0,
		ImageReferenceHTTPClient: server.Client(),
	})
	startedAt := time.Now()
	_, err := client.EditImage(context.Background(), &ImageEditRequest{
		Image:     []byte("primary-image"),
		ImageURLs: []string{server.URL + "/secondary.png"},
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
		t.Fatalf("EditImage() error = %v, want context deadline exceeded", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("secondary reference was not requested")
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("secondary download took %s, want client timeout", elapsed)
	}
}

func TestClientEditImageRejectsUnsafeSecondaryURL(t *testing.T) {
	client := NewClient(&ClientConfig{
		APIKey:     "test-key",
		Model:      "gpt-image-1",
		BaseURL:    "https://api.example.test/v1",
		Timeout:    time.Second,
		MaxRetries: 0,
	})
	_, err := client.EditImage(context.Background(), &ImageEditRequest{
		Image:     []byte("primary-image"),
		ImageURLs: []string{"http://127.0.0.1/internal.png"},
	})
	if err == nil || !strings.Contains(err.Error(), "validate secondary image URL") {
		t.Fatalf("EditImage() error = %v, want unsafe URL validation error", err)
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
