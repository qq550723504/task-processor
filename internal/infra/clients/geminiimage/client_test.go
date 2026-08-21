package geminiimage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type rewriteImageReferenceTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t rewriteImageReferenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	requestURL := *clone.URL
	requestURL.Scheme = t.target.Scheme
	requestURL.Host = t.target.Host
	clone.URL = &requestURL
	return t.base.RoundTrip(clone)
}

func imageReferenceClient(server *httptest.Server) *http.Client {
	target, _ := url.Parse(server.URL)
	return &http.Client{Transport: rewriteImageReferenceTransport{base: server.Client().Transport, target: target}}
}

func TestClientGenerateImageUsesGeminiGenerateContentEndpoint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-2.5-flash-image:generateContent" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
			t.Fatalf("x-goog-api-key = %q", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		config, _ := req["generationConfig"].(map[string]any)
		modalities, _ := config["responseModalities"].([]any)
		if len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
			t.Fatalf("responseModalities = %#v", modalities)
		}
		imageConfig, _ := config["imageConfig"].(map[string]any)
		if imageConfig["aspectRatio"] != "1:1" {
			t.Fatalf("aspectRatio = %#v", imageConfig["aspectRatio"])
		}
		if imageConfig["imageSize"] != "1K" {
			t.Fatalf("imageSize = %#v", imageConfig["imageSize"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"inlineData": map[string]any{
									"mimeType": "image/png",
									"data":     base64.StdEncoding.EncodeToString([]byte("generated-image")),
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:                   "test-key",
		Model:                    "gemini-2.5-flash-image",
		BaseURL:                  server.URL + "/v1beta",
		Timeout:                  time.Second,
		MaxAttempts:              1,
		HTTPClient:               server.Client(),
		ImageReferenceHTTPClient: imageReferenceClient(server),
	})

	resp, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "create a product hero image",
		Size:   "1024x1024",
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if resp.Data[0].B64JSON != base64.StdEncoding.EncodeToString([]byte("generated-image")) {
		t.Fatalf("b64_json = %q", resp.Data[0].B64JSON)
	}
}

func TestClientEditImageRejectsUnsafeSecondaryURL(t *testing.T) {
	client := NewClient(Config{Model: "gemini-2.5-flash-image", Timeout: time.Second})
	_, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt:           "edit faithfully",
		Image:            []byte("inline-primary"),
		ImageContentType: "image/png",
		ImageURLs:        []string{"http://127.0.0.1/internal.png"},
	})
	if err == nil || !strings.Contains(err.Error(), "validate source image URL") {
		t.Fatalf("EditImage() error = %v, want unsafe URL validation error", err)
	}
}

func TestClientEditImageDownloadsSourceURLsAndSendsInlineData(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/source.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("source-image"))
		case "/v1beta/models/gemini-2.5-flash-image:generateContent":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			contents, _ := req["contents"].([]any)
			if len(contents) != 1 {
				t.Fatalf("contents = %#v", contents)
			}
			entry, _ := contents[0].(map[string]any)
			parts, _ := entry["parts"].([]any)
			if len(parts) != 2 {
				t.Fatalf("parts = %#v", parts)
			}
			imagePart, _ := parts[0].(map[string]any)
			inlineData, _ := imagePart["inlineData"].(map[string]any)
			if inlineData["mimeType"] != "image/png" {
				t.Fatalf("mimeType = %#v", inlineData["mimeType"])
			}
			if inlineData["data"] != base64.StdEncoding.EncodeToString([]byte("source-image")) {
				t.Fatalf("inline data = %#v", inlineData["data"])
			}
			textPart, _ := parts[1].(map[string]any)
			if textPart["text"] != "make the background pure white" {
				t.Fatalf("text = %#v", textPart["text"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"candidates": []map[string]any{
					{
						"content": map[string]any{
							"parts": []map[string]any{
								{
									"inlineData": map[string]any{
										"mimeType": "image/png",
										"data":     base64.StdEncoding.EncodeToString([]byte("edited-image")),
									},
								},
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:                   "test-key",
		Model:                    "gemini-2.5-flash-image",
		BaseURL:                  server.URL + "/v1beta",
		Timeout:                  time.Second,
		MaxAttempts:              1,
		HTTPClient:               server.Client(),
		ImageReferenceHTTPClient: imageReferenceClient(server),
	})

	resp, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt:   "make the background pure white",
		ImageURL: "https://image.example.test/source.png",
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if resp.Data[0].B64JSON != base64.StdEncoding.EncodeToString([]byte("edited-image")) {
		t.Fatalf("b64_json = %q", resp.Data[0].B64JSON)
	}
}

func TestClientEditImageUsesInlineBytesWithoutDownloadingDuplicateURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/should-not-download.png" {
			t.Fatalf("duplicate source URL was downloaded")
		}
		if r.URL.Path != "/v1beta/models/gemini-2.5-flash-image:generateContent" {
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		contents, _ := req["contents"].([]any)
		parts, _ := contents[0].(map[string]any)["parts"].([]any)
		if len(parts) != 2 {
			t.Fatalf("parts = %#v, want inline image and text parts", parts)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{"parts": []map[string]any{{
					"inlineData": map[string]any{
						"mimeType": "image/png",
						"data":     base64.StdEncoding.EncodeToString([]byte("edited-image")),
					},
				}}},
			}},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:                   "test-key",
		Model:                    "gemini-2.5-flash-image",
		BaseURL:                  server.URL + "/v1beta",
		Timeout:                  time.Second,
		MaxAttempts:              1,
		HTTPClient:               server.Client(),
		ImageReferenceHTTPClient: imageReferenceClient(server),
	})

	resp, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt:           "edit faithfully",
		Image:            []byte("inline-image"),
		ImageContentType: "image/png",
		ImageURL:         "https://image.example.test/should-not-download.png",
		ImageURLs:        []string{"https://image.example.test/should-not-download.png"},
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
}

func TestClientEditImageIncludesSecondaryURLsAlongsideInlinePrimary(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/secondary.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("secondary-image"))
			return
		case "/v1beta/models/gemini-2.5-flash-image:generateContent":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			contents, _ := req["contents"].([]any)
			parts, _ := contents[0].(map[string]any)["parts"].([]any)
			if len(parts) != 3 {
				t.Fatalf("parts = %#v, want inline primary, secondary reference, and text", parts)
			}
			secondary, _ := parts[1].(map[string]any)["inlineData"].(map[string]any)
			if got := secondary["data"]; got != base64.StdEncoding.EncodeToString([]byte("secondary-image")) {
				t.Fatalf("secondary inline data = %#v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"candidates": []map[string]any{{"content": map[string]any{"parts": []map[string]any{{
					"inlineData": map[string]any{"mimeType": "image/png", "data": base64.StdEncoding.EncodeToString([]byte("edited-image"))},
				}}}}},
			})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:                   "test-key",
		Model:                    "gemini-2.5-flash-image",
		BaseURL:                  server.URL + "/v1beta",
		Timeout:                  time.Second,
		MaxAttempts:              1,
		HTTPClient:               server.Client(),
		ImageReferenceHTTPClient: imageReferenceClient(server),
	})
	if _, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt:           "edit faithfully",
		Image:            []byte("inline-primary"),
		ImageContentType: "image/png",
		ImageURLs:        []string{"https://image.example.test/secondary.png"},
	}); err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
}
