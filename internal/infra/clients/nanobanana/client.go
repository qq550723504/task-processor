package nanobanana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	MaxAttempts  int
	HTTPClient   *http.Client
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

type submitRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Image          []string `json:"image,omitempty"`
	Size           string   `json:"size,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"`
}

type submitResponse struct {
	ID      string       `json:"id"`
	Status  string       `json:"status"`
	Results []resultItem `json:"results"`
	Error   string       `json:"error"`
}

type resultPayload struct {
	ID       string       `json:"id"`
	Results  []resultItem `json:"results"`
	Progress int          `json:"progress"`
	Status   string       `json:"status"`
	Error    string       `json:"error"`
}

type resultItem struct {
	URL     string `json:"url"`
	Content string `json:"content"`
}

type gptImageGenerationsResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL string `json:"url"`
	} `json:"data"`
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
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
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
	return c.submitImagesGeneration(ctx, submitRequest{
		Model:          defaultString(req.Model, c.cfg.Model),
		Prompt:         req.Prompt,
		Size:           normalizeGenerationSize(req.Size),
		ResponseFormat: "url",
	})
}

func (c *Client) EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image edit request cannot be nil")
	}
	imageURLs := cleanImageURLs(req.ImageURLs, 8)
	if len(imageURLs) == 0 {
		imageURLs = cleanImageURLs([]string{req.ImageURL}, 1)
	}
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("nanobanana image edit requires image url")
	}
	return c.submitImagesGeneration(ctx, submitRequest{
		Model:          defaultString(req.Model, c.cfg.Model),
		Prompt:         req.Prompt,
		Size:           normalizeGenerationSize(req.Size),
		Image:          imageURLs,
		ResponseFormat: "url",
	})
}

func cleanImageURLs(urls []string, max int) []string {
	if max <= 0 {
		max = len(urls)
	}
	cleaned := make([]string, 0, min(len(urls), max))
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		cleaned = append(cleaned, item)
		if len(cleaned) >= max {
			break
		}
	}
	return cleaned
}

func (c *Client) submitImagesGeneration(ctx context.Context, req submitRequest) (*openaiclient.ImageResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("nanobanana model cannot be empty")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("nanobanana prompt cannot be empty")
	}
	if c.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	submitURL, err := buildSubmitURL(c.cfg.SubmitURL, req.Model)
	if err != nil {
		return nil, err
	}
	resultURL, err := buildResultURL(c.cfg.SubmitURL)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		result, err := c.submitGenerationRequest(ctx, submitURL, resultURL, req)
		if err == nil {
			return c.downloadGeneratedImages(ctx, result)
		}
		lastErr = err
		if !shouldRetryNanobananaError(lastErr) || attempt == c.cfg.MaxAttempts || ctx.Err() != nil {
			return nil, lastErr
		}
		select {
		case <-time.After(time.Duration(attempt) * c.cfg.PollInterval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (c *Client) submitGenerationRequest(ctx context.Context, submitURL string, resultURL string, req submitRequest) (*resultPayload, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal image generation request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("build image generation request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit image generation request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("submit image generation request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image generation response: %w", err)
	}
	var providerStatus submitResponse
	if err := json.Unmarshal(body, &providerStatus); err == nil {
		status := strings.ToLower(strings.TrimSpace(providerStatus.Status))
		switch status {
		case "failed", "violation":
			return nil, &JobError{
				Reason: strings.TrimSpace(providerStatus.Status),
				Detail: strings.TrimSpace(providerStatus.Error),
			}
		case "succeeded":
			return &resultPayload{
				ID:      providerStatus.ID,
				Status:  providerStatus.Status,
				Results: providerStatus.Results,
			}, nil
		}
		if isRunningStatus(status) && strings.TrimSpace(providerStatus.ID) != "" {
			return c.pollGenerationResult(ctx, resultURL, providerStatus.ID)
		}
	}
	var parsed gptImageGenerationsResponse
	if err := json.Unmarshal(body, &parsed); err == nil {
		results := make([]resultItem, 0, len(parsed.Data))
		for _, item := range parsed.Data {
			if strings.TrimSpace(item.URL) == "" {
				continue
			}
			results = append(results, resultItem{URL: item.URL})
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("image generation response contained no url")
		}
		return &resultPayload{
			Status:  "succeeded",
			Results: results,
		}, nil
	}
	var parsedResult resultPayload
	if err := json.Unmarshal(body, &parsedResult); err == nil {
		status := strings.ToLower(strings.TrimSpace(parsedResult.Status))
		switch status {
		case "failed", "violation":
			return nil, &JobError{
				Reason: strings.TrimSpace(parsedResult.Status),
				Detail: strings.TrimSpace(parsedResult.Error),
			}
		case "succeeded":
			if len(parsedResult.Results) == 0 {
				return nil, fmt.Errorf("image generation response contained no result")
			}
			return &parsedResult, nil
		}
		if isRunningStatus(status) && strings.TrimSpace(parsedResult.ID) != "" {
			return c.pollGenerationResult(ctx, resultURL, parsedResult.ID)
		}
	}
	return nil, fmt.Errorf("decode image generation response: unsupported response shape")
}

func (c *Client) pollGenerationResult(ctx context.Context, resultURL string, jobID string) (*resultPayload, error) {
	for {
		payload, err := c.fetchGenerationResult(ctx, resultURL, jobID)
		if err != nil {
			return nil, err
		}
		status := strings.ToLower(strings.TrimSpace(payload.Status))
		switch status {
		case "failed", "violation":
			return nil, &JobError{
				Reason: strings.TrimSpace(payload.Status),
				Detail: strings.TrimSpace(payload.Error),
			}
		case "succeeded":
			if len(payload.Results) == 0 {
				return nil, fmt.Errorf("image generation result contained no output")
			}
			return payload, nil
		}
		if !isRunningStatus(status) {
			return nil, fmt.Errorf("unexpected image generation status: %s", payload.Status)
		}
		select {
		case <-time.After(c.cfg.PollInterval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) fetchGenerationResult(ctx context.Context, resultURL string, jobID string) (*resultPayload, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build image generation result request: %w", err)
	}
	query := httpReq.URL.Query()
	query.Set("id", jobID)
	httpReq.URL.RawQuery = query.Encode()
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query image generation result: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("query image generation result returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var payload resultPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode image generation result: %w", err)
	}
	return &payload, nil
}

func (c *Client) downloadGeneratedImages(ctx context.Context, payload *resultPayload) (*openaiclient.ImageResponse, error) {
	if payload == nil || len(payload.Results) == 0 {
		return nil, fmt.Errorf("gpt image response contained no images")
	}
	data := make([]openaiclient.ImageData, 0, len(payload.Results))
	for _, item := range payload.Results {
		urlValue := strings.TrimSpace(item.URL)
		if urlValue != "" {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlValue, nil)
			if err != nil {
				return nil, fmt.Errorf("build download request: %w", err)
			}
			httpResp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("download gpt image: %w", err)
			}
			body, readErr := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read gpt image: %w", readErr)
			}
			if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
				return nil, fmt.Errorf("download gpt image returned status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
			}
			data = append(data, openaiclient.ImageData{
				URL:     urlValue,
				B64JSON: base64.StdEncoding.EncodeToString(body),
			})
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content != "" {
			data = append(data, openaiclient.ImageData{B64JSON: content})
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("gpt image response contained no downloadable url")
	}
	return &openaiclient.ImageResponse{Data: data}, nil
}

func buildSubmitURL(configuredURL string, model string) (string, error) {
	return buildGRSAIURL(configuredURL, "/v1/api/generate")
}

func buildResultURL(configuredURL string) (string, error) {
	return buildGRSAIURL(configuredURL, "/v1/api/result")
}

func buildGRSAIURL(configuredURL string, path string) (string, error) {
	trimmed := strings.TrimSpace(configuredURL)
	if trimmed == "" {
		return "", fmt.Errorf("nanobanana submit url cannot be empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse nanobanana submit url: %w", err)
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func isRunningStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "queued", "pending", "processing", "submitted", "in_progress":
		return true
	default:
		return false
	}
}

func normalizeGenerationSize(size string) string {
	trimmed := strings.TrimSpace(size)
	if strings.EqualFold(trimmed, "auto") {
		return ""
	}
	return trimmed
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func shouldRetryNanobananaError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	var jobErr *JobError
	if !strings.Contains(message, "output_moderation") &&
		!strings.Contains(message, "input_moderation") {
		if strings.Contains(message, "timeout") ||
			strings.Contains(message, "deadline exceeded") ||
			strings.Contains(message, "temporarily unavailable") ||
			strings.Contains(message, "connection reset") ||
			strings.Contains(message, "unexpected eof") ||
			strings.Contains(message, "internal error") {
			return true
		}
	}
	if !errors.As(err, &jobErr) {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(jobErr.Reason))
	detail := strings.ToLower(strings.TrimSpace(jobErr.Detail))
	if reason == "input_moderation" || reason == "output_moderation" {
		return false
	}
	return reason == "error" ||
		reason == "timeout" ||
		strings.Contains(detail, "timeout") ||
		strings.Contains(detail, "deadline exceeded") ||
		strings.Contains(detail, "temporarily unavailable") ||
		strings.Contains(detail, "overloaded") ||
		strings.Contains(detail, "internal error")
}
