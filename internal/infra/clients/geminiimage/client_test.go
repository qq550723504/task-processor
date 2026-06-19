package geminiimage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestClientGenerateImageUsesGeminiGenerateContentEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		APIKey:      "test-key",
		Model:       "gemini-2.5-flash-image",
		BaseURL:     server.URL + "/v1beta",
		Timeout:     time.Second,
		MaxAttempts: 1,
		HTTPClient:  server.Client(),
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

func TestClientEditImageDownloadsSourceURLsAndSendsInlineData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		APIKey:      "test-key",
		Model:       "gemini-2.5-flash-image",
		BaseURL:     server.URL + "/v1beta",
		Timeout:     time.Second,
		MaxAttempts: 1,
		HTTPClient:  server.Client(),
	})

	resp, err := client.EditImage(context.Background(), &openaiclient.ImageEditRequest{
		Prompt:   "make the background pure white",
		ImageURL: server.URL + "/source.png",
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
