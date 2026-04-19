package productimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/disintegration/imaging"
)

func TestHTTPSceneGenerationClientGenerateScene(t *testing.T) {
	rendered := imaging.New(1600, 1600, color.NRGBA{R: 240, G: 245, B: 250, A: 255})
	var imageBuf bytes.Buffer
	if err := jpeg.Encode(&imageBuf, rendered, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload sceneGenerationHTTPPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.SceneIntent != "gallery_scene" {
			t.Fatalf("scene intent = %q", payload.SceneIntent)
		}
		if payload.PromptRef != "productimage.scene.default" || payload.GenerationRef != "productimage.scene.default" {
			t.Fatalf("payload = %+v", payload)
		}
		resp := sceneGenerationHTTPResponse{
			Images: []struct {
				ImageBase64 string            `json:"image_base64"`
				Format      string            `json:"format"`
				Metadata    map[string]string `json:"metadata,omitempty"`
			}{
				{
					ImageBase64: base64.StdEncoding.EncodeToString(imageBuf.Bytes()),
					Format:      "jpeg",
					Metadata: map[string]string{
						"provider":        "scene-service",
						"model_family":    "gpt-image",
						"generation_mode": "scene_generation",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewHTTPSceneGenerationClient(HTTPSceneGenerationClientConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("NewHTTPSceneGenerationClient: %v", err)
	}
	results, err := client.GenerateScene(context.Background(), imageBuf.Bytes(), "https://example.com/source.jpg", SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		PromptRef:   "productimage.scene.default",
	})
	if err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Metadata["provider"] != "scene-service" {
		t.Fatalf("metadata = %+v", results[0].Metadata)
	}
}

func TestHTTPSceneGenerationClientNormalizesLegacyPromptRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload sceneGenerationHTTPPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.PromptRef != "productimage.scene.default" || payload.GenerationRef != "productimage.scene.default" {
			t.Fatalf("payload = %+v", payload)
		}
		_ = json.NewEncoder(w).Encode(sceneGenerationHTTPResponse{
			Images: []struct {
				ImageBase64 string            `json:"image_base64"`
				Format      string            `json:"format"`
				Metadata    map[string]string `json:"metadata,omitempty"`
			}{
				{
					ImageBase64: base64.StdEncoding.EncodeToString([]byte("fake")),
					Format:      "png",
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewHTTPSceneGenerationClient(HTTPSceneGenerationClientConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("NewHTTPSceneGenerationClient: %v", err)
	}

	_, err = client.GenerateScene(context.Background(), []byte("img"), "https://example.com/source.jpg", SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		PromptRef:   "productimage/scene/default",
	})
	if err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
}
