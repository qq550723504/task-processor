package nanobanana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type Config struct {
	APIKey       string
	Model        string
	SubmitURL    string
	PollInterval time.Duration
	Timeout      time.Duration
	HTTPClient   *http.Client
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

type submitRequest struct {
	Model        string   `json:"model"`
	Prompt       string   `json:"prompt"`
	AspectRatio  string   `json:"aspectRatio,omitempty"`
	ImageSize    string   `json:"imageSize,omitempty"`
	URLs         []string `json:"urls,omitempty"`
	WebHook      string   `json:"webHook,omitempty"`
	ShutProgress bool     `json:"shutProgress,omitempty"`
}

type submitResponse struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

type resultEnvelope struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data resultPayload `json:"data"`
}

type resultPayload struct {
	ID            string       `json:"id"`
	Results       []resultItem `json:"results"`
	Progress      int          `json:"progress"`
	Status        string       `json:"status"`
	FailureReason string       `json:"failure_reason"`
	Error         string       `json:"error"`
}

type resultItem struct {
	URL     string `json:"url"`
	Content string `json:"content"`
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 300 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	return &Client{cfg: cfg, httpClient: httpClient}
}

func (c *Client) GetDefaultModel() string {
	return c.cfg.Model
}

func (c *Client) GenerateImage(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image generate request cannot be nil")
	}
	return c.submitAndPoll(ctx, submitRequest{
		Model:        defaultString(req.Model, c.cfg.Model),
		Prompt:       req.Prompt,
		AspectRatio:  nanoBananaAspectRatio(req.Size),
		ImageSize:    nanoBananaImageSize(req.Size),
		WebHook:      "-1",
		ShutProgress: true,
	})
}

func (c *Client) EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image edit request cannot be nil")
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		return nil, fmt.Errorf("nanobanana image edit requires image url")
	}
	return c.submitAndPoll(ctx, submitRequest{
		Model:        defaultString(req.Model, c.cfg.Model),
		Prompt:       req.Prompt,
		AspectRatio:  nanoBananaAspectRatio(req.Size),
		ImageSize:    nanoBananaImageSize(req.Size),
		URLs:         []string{req.ImageURL},
		WebHook:      "-1",
		ShutProgress: true,
	})
}

func (c *Client) submitAndPoll(ctx context.Context, req submitRequest) (*openaiclient.ImageResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("nanobanana model cannot be empty")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("nanobanana prompt cannot be empty")
	}

	jobID, err := c.submit(ctx, req)
	if err != nil {
		return nil, err
	}
	result, err := c.poll(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return c.toImageResponse(ctx, result)
}

func (c *Client) submit(ctx context.Context, req submitRequest) (string, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal submit request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.SubmitURL, strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("build submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("submit nanobanana job: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("submit nanobanana job returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var parsed submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode submit response: %w", err)
	}
	if parsed.Code != 0 {
		return "", fmt.Errorf("submit nanobanana job failed: %s", parsed.Msg)
	}
	if strings.TrimSpace(parsed.Data.ID) == "" {
		return "", fmt.Errorf("submit nanobanana job returned empty id")
	}
	return parsed.Data.ID, nil
}

func (c *Client) poll(ctx context.Context, id string) (*resultPayload, error) {
	resultURL, err := buildResultURL(c.cfg.SubmitURL)
	if err != nil {
		return nil, err
	}
	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()

	for {
		payload, err := json.Marshal(map[string]string{"id": id})
		if err != nil {
			return nil, fmt.Errorf("marshal poll request: %w", err)
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, resultURL, strings.NewReader(string(payload)))
		if err != nil {
			return nil, fmt.Errorf("build poll request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if strings.TrimSpace(c.cfg.APIKey) != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("poll nanobanana job: %w", err)
		}
		var parsed resultEnvelope
		decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode poll response: %w", decodeErr)
		}
		if parsed.Code != 0 {
			return nil, fmt.Errorf("poll nanobanana job failed: %s", parsed.Msg)
		}

		switch parsed.Data.Status {
		case "succeeded":
			return &parsed.Data, nil
		case "failed":
			return nil, fmt.Errorf("nanobanana job failed: %s (%s)", parsed.Data.FailureReason, parsed.Data.Error)
		case "running", "":
		default:
			return nil, fmt.Errorf("unexpected nanobanana job status: %s", parsed.Data.Status)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("poll nanobanana job canceled: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) toImageResponse(ctx context.Context, result *resultPayload) (*openaiclient.ImageResponse, error) {
	if result == nil || len(result.Results) == 0 {
		return nil, fmt.Errorf("nanobanana result contained no images")
	}
	first := result.Results[0]
	if strings.TrimSpace(first.URL) == "" {
		return nil, fmt.Errorf("nanobanana result missing image url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, first.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download nanobanana image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download nanobanana image returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read nanobanana image: %w", err)
	}
	return &openaiclient.ImageResponse{
		Data: []openaiclient.ImageData{
			{
				URL:           first.URL,
				B64JSON:       base64.StdEncoding.EncodeToString(data),
				RevisedPrompt: first.Content,
			},
		},
	}, nil
}

func buildResultURL(submitURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(submitURL))
	if err != nil {
		return "", fmt.Errorf("parse nanobanana submit url: %w", err)
	}
	parsed.Path = path.Dir(parsed.Path) + "/result"
	return parsed.String(), nil
}

func nanoBananaAspectRatio(size string) string {
	switch strings.TrimSpace(size) {
	case "1024x1024", "1536x1536", "2048x2048":
		return "1:1"
	default:
		return "auto"
	}
}

func nanoBananaImageSize(size string) string {
	switch strings.TrimSpace(size) {
	case "2048x2048":
		return "2K"
	case "4096x4096":
		return "4K"
	default:
		return "1K"
	}
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
