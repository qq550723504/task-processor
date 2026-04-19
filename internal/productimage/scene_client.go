package productimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type SceneImageResult struct {
	ImageData []byte
	Format    string
	Metadata  map[string]string
}

type SceneGenerationClient interface {
	GenerateScene(ctx context.Context, imageData []byte, sourceURL string, request SceneGenerationRequest) ([]SceneImageResult, error)
}

type HTTPSceneGenerationClientConfig struct {
	Endpoint   string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type httpSceneGenerationClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

type sceneGenerationHTTPPayload struct {
	ImageBase64   string            `json:"image_base64"`
	SourceURL     string            `json:"source_url,omitempty"`
	SceneIntent   string            `json:"scene_intent,omitempty"`
	PromptRef     string            `json:"prompt_ref,omitempty"`
	ProductType   string            `json:"product_type,omitempty"`
	Title         string            `json:"title,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	GenerationRef string            `json:"generation_ref,omitempty"`
}

type sceneGenerationHTTPResponse struct {
	Images []struct {
		ImageBase64 string            `json:"image_base64"`
		Format      string            `json:"format"`
		Metadata    map[string]string `json:"metadata,omitempty"`
	} `json:"images"`
	Error string `json:"error,omitempty"`
}

func NewHTTPSceneGenerationClient(config HTTPSceneGenerationClientConfig) (SceneGenerationClient, error) {
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, fmt.Errorf("scene generation endpoint cannot be empty")
	}
	client := config.HTTPClient
	if client == nil {
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &httpSceneGenerationClient{
		endpoint:   strings.TrimSpace(config.Endpoint),
		apiKey:     strings.TrimSpace(config.APIKey),
		httpClient: client,
	}, nil
}

func (c *httpSceneGenerationClient) GenerateScene(ctx context.Context, imageData []byte, sourceURL string, request SceneGenerationRequest) ([]SceneImageResult, error) {
	if len(imageData) == 0 {
		return nil, fmt.Errorf("image data cannot be empty")
	}
	normalizedPromptRef := normalizedScenePromptRef(&request)
	payload := sceneGenerationHTTPPayload{
		ImageBase64:   base64.StdEncoding.EncodeToString(imageData),
		SourceURL:     sourceURL,
		SceneIntent:   request.SceneIntent,
		PromptRef:     normalizedPromptRef,
		GenerationRef: normalizedPromptRef,
	}
	if request.ProductContext != nil {
		payload.ProductType = request.ProductContext.ProductType
		payload.Title = request.ProductContext.Title
		payload.Attributes = request.ProductContext.Attributes
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal scene generation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create scene generation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send scene generation request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scene generation service returned status %d", resp.StatusCode)
	}
	var parsed sceneGenerationHTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode scene generation response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("scene generation service error: %s", parsed.Error)
	}
	if len(parsed.Images) == 0 {
		return nil, fmt.Errorf("scene generation response missing images")
	}
	results := make([]SceneImageResult, 0, len(parsed.Images))
	for _, image := range parsed.Images {
		if image.ImageBase64 == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(image.ImageBase64)
		if err != nil {
			return nil, fmt.Errorf("decode scene generation image: %w", err)
		}
		results = append(results, SceneImageResult{
			ImageData: decoded,
			Format:    image.Format,
			Metadata:  image.Metadata,
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("scene generation response contained no decodable images")
	}
	return results, nil
}
