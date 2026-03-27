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

type SegmentationResult struct {
	ImageData []byte
	Format    string
	BBox      string
	Metadata  map[string]string
}

type SegmentationClient interface {
	SegmentSubject(ctx context.Context, imageData []byte, sourceURL string) (*SegmentationResult, error)
}

type HTTPSegmentationClientConfig struct {
	Endpoint   string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type httpSegmentationClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func NewHTTPSegmentationClient(config HTTPSegmentationClientConfig) (SegmentationClient, error) {
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, fmt.Errorf("segmentation endpoint cannot be empty")
	}
	client := config.HTTPClient
	if client == nil {
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 45 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &httpSegmentationClient{
		endpoint:   strings.TrimSpace(config.Endpoint),
		apiKey:     strings.TrimSpace(config.APIKey),
		httpClient: client,
	}, nil
}

type segmentationRequest struct {
	ImageBase64 string `json:"image_base64"`
	SourceURL   string `json:"source_url,omitempty"`
	Task        string `json:"task"`
}

type segmentationResponse struct {
	ImageBase64 string            `json:"image_base64"`
	Format      string            `json:"format"`
	BBox        string            `json:"bbox,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
}

func (c *httpSegmentationClient) SegmentSubject(ctx context.Context, imageData []byte, sourceURL string) (*SegmentationResult, error) {
	if len(imageData) == 0 {
		return nil, fmt.Errorf("image data cannot be empty")
	}
	body, err := json.Marshal(segmentationRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(imageData),
		SourceURL:   sourceURL,
		Task:        "subject_extract",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal segmentation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create segmentation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send segmentation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("segmentation service returned status %d", resp.StatusCode)
	}

	var payload segmentationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode segmentation response: %w", err)
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("segmentation service error: %s", payload.Error)
	}
	if payload.ImageBase64 == "" {
		return nil, fmt.Errorf("segmentation response missing image_base64")
	}

	decoded, err := base64.StdEncoding.DecodeString(payload.ImageBase64)
	if err != nil {
		return nil, fmt.Errorf("decode segmentation image: %w", err)
	}
	return &SegmentationResult{
		ImageData: decoded,
		Format:    payload.Format,
		BBox:      payload.BBox,
		Metadata:  payload.Metadata,
	}, nil
}
