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

type WhiteBackgroundResult struct {
	ImageData []byte
	Format    string
	Metadata  map[string]string
}

type WhiteBackgroundClient interface {
	RenderWhiteBackground(ctx context.Context, imageData []byte, sourceURL string) (*WhiteBackgroundResult, error)
}

type HTTPWhiteBackgroundClientConfig struct {
	Endpoint   string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type httpWhiteBackgroundClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func NewHTTPWhiteBackgroundClient(config HTTPWhiteBackgroundClientConfig) (WhiteBackgroundClient, error) {
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, fmt.Errorf("white background endpoint cannot be empty")
	}
	client := config.HTTPClient
	if client == nil {
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 45 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &httpWhiteBackgroundClient{
		endpoint:   strings.TrimSpace(config.Endpoint),
		apiKey:     strings.TrimSpace(config.APIKey),
		httpClient: client,
	}, nil
}

type whiteBackgroundRequest struct {
	ImageBase64 string `json:"image_base64"`
	SourceURL   string `json:"source_url,omitempty"`
	Task        string `json:"task"`
}

type whiteBackgroundResponse struct {
	ImageBase64 string            `json:"image_base64"`
	Format      string            `json:"format"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
}

func (c *httpWhiteBackgroundClient) RenderWhiteBackground(ctx context.Context, imageData []byte, sourceURL string) (*WhiteBackgroundResult, error) {
	if len(imageData) == 0 {
		return nil, fmt.Errorf("image data cannot be empty")
	}
	body, err := json.Marshal(whiteBackgroundRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(imageData),
		SourceURL:   sourceURL,
		Task:        "white_background",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal white background request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create white background request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send white background request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("white background service returned status %d", resp.StatusCode)
	}

	var payload whiteBackgroundResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode white background response: %w", err)
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("white background service error: %s", payload.Error)
	}
	if payload.ImageBase64 == "" {
		return nil, fmt.Errorf("white background response missing image_base64")
	}

	decoded, err := base64.StdEncoding.DecodeString(payload.ImageBase64)
	if err != nil {
		return nil, fmt.Errorf("decode white background image: %w", err)
	}
	return &WhiteBackgroundResult{
		ImageData: decoded,
		Format:    payload.Format,
		Metadata:  payload.Metadata,
	}, nil
}
