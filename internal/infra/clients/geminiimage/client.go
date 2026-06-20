package geminiimage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type Config struct {
	APIKey      string
	Model       string
	BaseURL     string
	Timeout     time.Duration
	MaxAttempts int
	RetryDelay  time.Duration
	HTTPClient  *http.Client
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

type generateContentRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	ResponseModalities []string           `json:"responseModalities,omitempty"`
	ImageConfig        *geminiImageConfig `json:"imageConfig,omitempty"`
}

type geminiImageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

type generateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 300 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{cfg: cfg, httpClient: httpClient}
}

func (c *Client) GetDefaultModel() string {
	return c.cfg.Model
}

func (c *Client) SupportsAsyncImageGeneration() bool {
	return false
}

func (c *Client) SubmitImageGeneration(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (c *Client) SubmitImageEdit(context.Context, *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (c *Client) QueryImageGeneration(context.Context, string) (*openaiclient.ImageAsyncQueryResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (c *Client) GenerateImage(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image generate request cannot be nil")
	}
	model := firstNonEmpty(req.Model, c.cfg.Model)
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("gemini image model cannot be empty")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("gemini image prompt cannot be empty")
	}
	return c.generateContent(ctx, model, []geminiPart{{Text: req.Prompt}}, req.Size)
}

func (c *Client) EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image edit request cannot be nil")
	}
	model := firstNonEmpty(req.Model, c.cfg.Model)
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("gemini image model cannot be empty")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("gemini image prompt cannot be empty")
	}
	imageParts, err := c.buildImageInputParts(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(imageParts) == 0 {
		return nil, fmt.Errorf("gemini image edit requires image bytes or downloadable image urls")
	}
	parts := append(imageParts, geminiPart{Text: req.Prompt})
	return c.generateContent(ctx, model, parts, req.Size)
}

func (c *Client) generateContent(ctx context.Context, model string, parts []geminiPart, size string) (*openaiclient.ImageResponse, error) {
	requestBody := generateContentRequest{
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: geminiGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}
	if imageConfig := sizeToGeminiImageConfig(size); imageConfig != nil {
		requestBody.GenerationConfig.ImageConfig = imageConfig
	}

	endpoint, err := buildGenerateContentURL(c.cfg.BaseURL, model)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < c.cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(c.cfg.RetryDelay * time.Duration(1<<uint(attempt-1))):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, err := c.doGenerateContent(ctx, endpoint, requestBody)
		if err == nil {
			return parseGenerateContentResponse(resp)
		}
		lastErr = err
		if !shouldRetry(err) {
			break
		}
	}
	return nil, fmt.Errorf("call Gemini image API failed after %d attempts: %w", c.cfg.MaxAttempts, lastErr)
}

func (c *Client) doGenerateContent(ctx context.Context, endpoint string, payload generateContentRequest) (*generateContentResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("build gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		req.Header.Set("x-goog-api-key", c.cfg.APIKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit gemini request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("gemini image api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var parsed generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}
	return &parsed, nil
}

func (c *Client) buildImageInputParts(ctx context.Context, req *openaiclient.ImageEditRequest) ([]geminiPart, error) {
	partCap := len(req.ImageURLs) + 1
	if partCap < 1 {
		partCap = 1
	}
	parts := make([]geminiPart, 0, partCap)
	if len(req.Image) > 0 {
		parts = append(parts, geminiPart{InlineData: &geminiInlineData{
			MIMEType: http.DetectContentType(req.Image),
			Data:     base64.StdEncoding.EncodeToString(req.Image),
		}})
	}
	for _, rawURL := range append([]string{req.ImageURL}, req.ImageURLs...) {
		imageURL := strings.TrimSpace(rawURL)
		if imageURL == "" {
			continue
		}
		data, mimeType, err := c.downloadSourceImage(ctx, imageURL)
		if err != nil {
			return nil, err
		}
		parts = append(parts, geminiPart{InlineData: &geminiInlineData{
			MIMEType: mimeType,
			Data:     base64.StdEncoding.EncodeToString(data),
		}})
	}
	return dedupeInlineParts(parts), nil
}

func (c *Client) downloadSourceImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build source image request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download source image: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read source image: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download source image returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = http.DetectContentType(body)
	}
	return body, mimeType, nil
}

func parseGenerateContentResponse(resp *generateContentResponse) (*openaiclient.ImageResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("gemini response cannot be nil")
	}
	data := make([]openaiclient.ImageData, 0)
	for _, candidate := range resp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || strings.TrimSpace(part.InlineData.Data) == "" {
				continue
			}
			data = append(data, openaiclient.ImageData{B64JSON: strings.TrimSpace(part.InlineData.Data)})
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("gemini response contained no generated image")
	}
	return &openaiclient.ImageResponse{Data: data}, nil
}

func buildGenerateContentURL(baseURL string, model string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse gemini base url: %w", err)
	}
	cleanPath := path.Clean("/" + strings.Trim(strings.TrimSpace(parsed.Path), "/"))
	if cleanPath == "/" || cleanPath == "." {
		cleanPath = "/v1beta"
	}
	if !strings.Contains(cleanPath, "/v1") {
		cleanPath = path.Join(cleanPath, "v1beta")
	}
	parsed.Path = path.Join(cleanPath, "models", model+":generateContent")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func sizeToGeminiImageConfig(size string) *geminiImageConfig {
	width, height, ok := parseImageSize(size)
	if !ok {
		return nil
	}
	cfg := &geminiImageConfig{
		AspectRatio: nearestAspectRatio(width, height),
		ImageSize:   nearestImageSize(width, height),
	}
	if cfg.AspectRatio == "" && cfg.ImageSize == "" {
		return nil
	}
	return cfg
}

func parseImageSize(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func nearestAspectRatio(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	type ratioOption struct {
		label string
		value float64
	}
	options := []ratioOption{
		{label: "1:1", value: 1},
		{label: "4:3", value: 4.0 / 3.0},
		{label: "3:4", value: 3.0 / 4.0},
		{label: "3:2", value: 3.0 / 2.0},
		{label: "2:3", value: 2.0 / 3.0},
		{label: "16:9", value: 16.0 / 9.0},
		{label: "9:16", value: 9.0 / 16.0},
	}
	target := float64(width) / float64(height)
	best := options[0]
	bestDiff := absFloat(target - best.value)
	for _, option := range options[1:] {
		diff := absFloat(target - option.value)
		if diff < bestDiff {
			best = option
			bestDiff = diff
		}
	}
	return best.label
}

func nearestImageSize(width int, height int) string {
	longest := width
	if height > longest {
		longest = height
	}
	switch {
	case longest >= 4096:
		return "4K"
	case longest >= 2048:
		return "2K"
	case longest >= 1024:
		return "1K"
	case longest > 0:
		return "512"
	default:
		return ""
	}
}

func dedupeInlineParts(parts []geminiPart) []geminiPart {
	seen := make(map[string]struct{}, len(parts))
	result := make([]geminiPart, 0, len(parts))
	for _, part := range parts {
		if part.InlineData == nil {
			result = append(result, part)
			continue
		}
		key := part.InlineData.MIMEType + ":" + part.InlineData.Data
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, part)
	}
	return result
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "temporarily unavailable") ||
		strings.Contains(message, "internal error") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "unexpected eof")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
