package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestImageResponseDecodesExplicitTokenUsage(t *testing.T) {
	var chat ChatCompletionResponse
	if err := json.Unmarshal([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`), &chat); err != nil || chat.Usage != (Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}) {
		t.Fatalf("chat usage schema changed: %+v, %v", chat.Usage, err)
	}
	for _, test := range []struct {
		name, usage string
		known       bool
	}{
		{"valid", `{"input_tokens":1,"output_tokens":3568,"total_tokens":3569}`, true},
		{"zero_input", `{"input_tokens":0,"output_tokens":3569,"total_tokens":3569}`, true},
		{"missing", "", false},
		{"null", "null", false},
		{"partial", `{"total_tokens":3569}`, false},
		{"null_counter", `{"input_tokens":null,"output_tokens":3569,"total_tokens":3569}`, false},
		{"negative", `{"input_tokens":-1,"output_tokens":3570,"total_tokens":3569}`, false},
		{"mismatch", `{"input_tokens":1,"output_tokens":2,"total_tokens":3569}`, false},
		{"zero_total", `{"input_tokens":0,"output_tokens":0,"total_tokens":0}`, false},
		{"overflow", `{"input_tokens":9223372036854775807,"output_tokens":1,"total_tokens":9223372036854775807}`, false},
		{"out_of_range", `{"input_tokens":9223372036854775808,"output_tokens":1,"total_tokens":9223372036854775809}`, false},
		{"fractional", `{"input_tokens":0.5,"output_tokens":3568.5,"total_tokens":3569}`, false},
		{"chat_schema", `{"prompt_tokens":1,"completion_tokens":3568,"total_tokens":3569}`, false},
	} {
		for _, edit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/edit=%t", test.name, edit), func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					body := `{"created":1,"data":[{"b64_json":"aW1hZ2U="}]`
					if test.usage != "" {
						body += `,"usage":` + test.usage
					}
					_, _ = w.Write([]byte(body + `}`))
				}))
				defer server.Close()
				client := NewClient(&ClientConfig{BaseURL: server.URL, Model: "gpt-image-2.5", Timeout: time.Second, MaxRetries: 0})
				var response *ImageResponse
				var err error
				if edit {
					zero := 0
					response, err = client.EditImage(context.Background(), &ImageEditRequest{Prompt: "white background", Image: []byte("source"), MaxRetries: &zero})
				} else {
					response, err = client.GenerateImage(context.Background(), &ImageGenerateRequest{Prompt: "white background"})
				}
				if err != nil || response == nil {
					t.Fatalf("image response lost: %v", err)
				}
				if response.UsageKnown != test.known {
					t.Errorf("UsageKnown=%t, want %t", response.UsageKnown, test.known)
				}
				if test.known {
					wantInput := 1
					if test.name == "zero_input" {
						wantInput = 0
					}
					if response.Usage != (Usage{PromptTokens: wantInput, CompletionTokens: 3569 - wantInput, TotalTokens: 3569}) {
						t.Errorf("image counters not mapped: %+v", response.Usage)
					}
				} else if response.Usage != (Usage{}) {
					t.Errorf("untrusted counters exposed as normalized usage: %+v", response.Usage)
				}
				if calls != 1 || len(response.Data) != 1 {
					t.Fatalf("calls=%d, image count=%d", calls, len(response.Data))
				}
			})
		}
	}
}

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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"usage": map[string]int{"input_tokens": 1, "output_tokens": 320, "total_tokens": 321},
			"data":  []ImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("pngdata"))}},
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
	if !resp.UsageKnown || resp.Usage.TotalTokens != 321 {
		t.Fatalf("usage = %+v, want total_tokens=321", resp.Usage)
	}
	if !strings.Contains(resp.RawResponse, "\"b64_json\"") {
		t.Fatalf("raw response = %q, want encoded image payload", resp.RawResponse)
	}
}

func TestClientCapsAggregateReferenceMaterialization(t *testing.T) {
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reference.png":
			entered <- struct{}{}
			select {
			case <-release:
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write([]byte("reference-image"))
			case <-r.Context().Done():
			}
		case "/images/edits":
			_ = json.NewEncoder(w).Encode(ImageResponse{Data: []ImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("edited"))}}})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		APIKey: "test-key", Model: "gpt-image-1", BaseURL: server.URL, Timeout: time.Second, MaxRetries: 0,
		ImageReferenceHTTPClient:      server.Client(),
		MaxReferenceMaterializedBytes: 1024 << 20, MaxReferenceMaterializationConcurrency: 8,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wait sync.WaitGroup
	errs := make(chan error, 9)
	for i := 0; i < 9; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := client.EditImage(ctx, &ImageEditRequest{
				Image: []byte("primary-image"), ImageContentType: "image/png",
				ImageURLs: []string{server.URL + "/reference.png"},
			})
			errs <- err
		}()
	}

	deadline := time.Now().Add(time.Second)
	for len(entered) < 8 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(entered) < 8 {
		t.Fatalf("concurrent references entered = %d, want 8", len(entered))
	}
	time.Sleep(50 * time.Millisecond)
	if got := len(entered); got > 8 {
		t.Fatalf("concurrent references entered = %d, want shared cap of 8", got)
	}
	close(release)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("EditImage() error = %v", err)
		}
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

func TestClientEditImageMaterializesPrimaryURLWhenBytesAreAbsent(t *testing.T) {
	primaryRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/primary.png":
			primaryRequests++
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("primary-image"))
			return
		case "/secondary.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("secondary-image"))
			return
		case "/images/edits":
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader: %v", err)
			}
			var images [][]byte
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("NextPart: %v", err)
				}
				if part.FormName() == "image[]" {
					data, err := io.ReadAll(part)
					if err != nil {
						t.Fatalf("read image part: %v", err)
					}
					images = append(images, data)
				}
			}
			if len(images) != 2 || string(images[0]) != "primary-image" || string(images[1]) != "secondary-image" {
				t.Fatalf("multipart images = %q, want [primary-image secondary-image]", images)
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
	primaryURL := server.URL + "/primary.png"
	if _, err := client.EditImage(context.Background(), &ImageEditRequest{
		ImageURL:  primaryURL,
		ImageURLs: []string{primaryURL, server.URL + "/secondary.png"},
	}); err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if primaryRequests != 1 {
		t.Fatalf("primary URL requests = %d, want 1", primaryRequests)
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

func TestClientEditImageRejectsOversizedSecondaryReference(t *testing.T) {
	client := NewClient(&ClientConfig{
		APIKey:     "test-key",
		Model:      "gpt-image-1",
		BaseURL:    "https://api.example.test/v1",
		Timeout:    time.Second,
		MaxRetries: 0,
		ImageReferenceHTTPClient: &http.Client{Transport: openAIImageReferenceRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(&fixedSizeReader{remaining: maxImageReferenceBytes + 1}),
			}, nil
		})},
	})
	_, err := client.EditImage(context.Background(), &ImageEditRequest{
		Image:     []byte("primary-image"),
		ImageURLs: []string{"https://example.test/secondary.png"},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds 32 MiB") {
		t.Fatalf("EditImage() error = %v, want oversized reference error", err)
	}
}

type openAIImageReferenceRoundTripper func(*http.Request) (*http.Response, error)

func (f openAIImageReferenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fixedSizeReader struct {
	remaining int64
}

func (r *fixedSizeReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 'x'
	}
	r.remaining -= n
	return int(n), nil
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
