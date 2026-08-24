package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"task-processor/internal/pkg/safeimagehttp"
)

const (
	maxImageReferenceBytes                     int64 = 32 << 20
	defaultReferenceMaterializedBytes          int64 = 512 << 20
	defaultReferenceMaterializationConcurrency       = 8
	referenceBudgetUnitBytes                   int64 = 1 << 20
)

type referenceMaterializationBudget struct {
	downloadSlots *semaphore.Weighted
	bytes         *semaphore.Weighted
	byteUnits     int64
}

func newReferenceMaterializationBudget(maxBytes int64, maxConcurrent int) *referenceMaterializationBudget {
	if maxBytes <= 0 {
		maxBytes = defaultReferenceMaterializedBytes
	}
	if maxBytes < maxImageReferenceBytes {
		maxBytes = maxImageReferenceBytes
	}
	if maxConcurrent <= 0 {
		maxConcurrent = defaultReferenceMaterializationConcurrency
	}
	units := (maxBytes + referenceBudgetUnitBytes - 1) / referenceBudgetUnitBytes
	return &referenceMaterializationBudget{
		downloadSlots: semaphore.NewWeighted(int64(maxConcurrent)),
		bytes:         semaphore.NewWeighted(units),
		byteUnits:     units,
	}
}

func (b *referenceMaterializationBudget) acquire(ctx context.Context, bytes int64) (func(), error) {
	if b == nil {
		return func() {}, nil
	}
	units := (bytes + referenceBudgetUnitBytes - 1) / referenceBudgetUnitBytes
	if units <= 0 || units > b.byteUnits {
		return nil, fmt.Errorf("openai reference materialization budget is too small")
	}
	if err := b.bytes.Acquire(ctx, units); err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { b.bytes.Release(units) }) }, nil
}

func (b *referenceMaterializationBudget) acquireDownload(ctx context.Context) (func(), error) {
	if b == nil {
		return func() {}, nil
	}
	if err := b.downloadSlots.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { b.downloadSlots.Release(1) }) }, nil
}

func extractImageRequestID(header http.Header) string {
	return strings.TrimSpace(header.Get("X-Request-Id"))
}

func (c *Client) GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error) {
	if c.pool == nil {
		return nil, fmt.Errorf("请求池未初始化")
	}
	return c.pool.GenerateImage(ctx, req)
}

func (c *Client) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	if c.pool == nil {
		return nil, fmt.Errorf("请求池未初始化")
	}
	return c.pool.EditImage(ctx, req)
}

func (c *Client) SupportsAsyncImageGeneration() bool {
	return false
}

func (c *Client) SubmitImageGeneration(context.Context, *ImageGenerateRequest) (*ImageAsyncSubmitResponse, error) {
	return nil, ErrAsyncImageGenerationNotSupported
}

func (c *Client) SubmitImageEdit(context.Context, *ImageEditRequest) (*ImageAsyncSubmitResponse, error) {
	return nil, ErrAsyncImageGenerationNotSupported
}

func (c *Client) QueryImageGeneration(context.Context, string) (*ImageAsyncQueryResponse, error) {
	return nil, ErrAsyncImageGenerationNotSupported
}

func (p *RequestPool) GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error) {
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, fmt.Errorf("等待并发槽位时上下文取消: %w", ctx.Err())
	}
	client := p.getNextClient()
	return client.generateImage(ctx, req)
}

func (p *RequestPool) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, fmt.Errorf("等待并发槽位时上下文取消: %w", ctx.Err())
	}
	client := p.getNextClient()
	return client.editImage(ctx, req)
}

func (bc *BaseClient) generateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image generate request cannot be nil")
	}
	payload := *req
	if payload.Model == "" {
		payload.Model = bc.config.Model
	}
	if payload.ResponseFormat == "" {
		payload.ResponseFormat = "b64_json"
	}
	if payload.N == 0 {
		payload.N = 1
	}
	return bc.doJSONImageRequest(ctx, http.MethodPost, "/images/generations", payload)
}

func (bc *BaseClient) editImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image edit request cannot be nil")
	}
	primaryURL := strings.TrimSpace(req.ImageURL)
	if len(req.Image) == 0 && primaryURL == "" {
		return nil, fmt.Errorf("image edit request requires image bytes or image URL")
	}
	primaryImage := req.Image
	primaryContentType := req.ImageContentType
	referenceURLs := make([]string, 0, len(req.ImageURLs)+1)
	seenURLs := map[string]struct{}{}
	if primaryURL != "" {
		seenURLs[primaryURL] = struct{}{}
	}
	if len(primaryImage) == 0 && primaryURL != "" {
		referenceURLs = append(referenceURLs, primaryURL)
	}
	for _, rawURL := range req.ImageURLs {
		imageURL := strings.TrimSpace(rawURL)
		if imageURL == "" {
			continue
		}
		if _, seen := seenURLs[imageURL]; seen {
			continue
		}
		seenURLs[imageURL] = struct{}{}
		referenceURLs = append(referenceURLs, imageURL)
	}
	if len(referenceURLs) > 0 {
		lease, err := bc.referenceMaterialization.acquire(ctx, int64(len(referenceURLs))*maxImageReferenceBytes)
		if err != nil {
			return nil, fmt.Errorf("reserve OpenAI image reference materialization budget: %w", err)
		}
		defer lease()
	}
	if len(primaryImage) == 0 {
		data, contentType, err := bc.downloadImageEditReference(ctx, referenceURLs[0])
		if err != nil {
			return nil, err
		}
		primaryImage = data
		primaryContentType = contentType
	}
	model := req.Model
	if model == "" {
		model = bc.config.Model
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("model", model)
	_ = writer.WriteField("prompt", req.Prompt)
	if req.Size != "" {
		_ = writer.WriteField("size", req.Size)
	}
	if req.Quality != "" {
		_ = writer.WriteField("quality", req.Quality)
	}
	responseFormat := req.ResponseFormat
	if responseFormat == "" {
		responseFormat = "b64_json"
	}
	_ = writer.WriteField("response_format", responseFormat)
	if req.N > 0 {
		_ = writer.WriteField("n", fmt.Sprintf("%d", req.N))
	}
	imagePart, err := writer.CreateFormFile("image[]", imageEditFilename(primaryContentType))
	if err != nil {
		return nil, fmt.Errorf("create image form file: %w", err)
	}
	if _, err := imagePart.Write(primaryImage); err != nil {
		return nil, fmt.Errorf("write image form file: %w", err)
	}
	secondaryStart := 0
	if len(req.Image) == 0 && primaryURL != "" && len(referenceURLs) > 0 && referenceURLs[0] == primaryURL {
		secondaryStart = 1
	}
	for _, imageURL := range referenceURLs[secondaryStart:] {
		data, contentType, err := bc.downloadImageEditReference(ctx, imageURL)
		if err != nil {
			return nil, err
		}
		secondaryPart, err := writer.CreateFormFile("image[]", imageEditFilename(contentType))
		if err != nil {
			return nil, fmt.Errorf("create secondary image form file: %w", err)
		}
		if _, err := secondaryPart.Write(data); err != nil {
			return nil, fmt.Errorf("write secondary image form file: %w", err)
		}
	}
	if len(req.Mask) > 0 {
		maskPart, err := writer.CreateFormFile("mask", "mask.png")
		if err != nil {
			return nil, fmt.Errorf("create mask form file: %w", err)
		}
		if _, err := maskPart.Write(req.Mask); err != nil {
			return nil, fmt.Errorf("write mask form file: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}
	return bc.doMultipartImageRequest(ctx, "/images/edits", body, writer.FormDataContentType())
}

func (bc *BaseClient) downloadImageEditReference(ctx context.Context, imageURL string) ([]byte, string, error) {
	downloadRelease, err := bc.referenceMaterialization.acquireDownload(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("reserve OpenAI image reference download slot: %w", err)
	}
	defer downloadRelease()
	downloadCtx := ctx
	cancel := func() {}
	if bc.config != nil && bc.config.Timeout > 0 {
		downloadCtx, cancel = context.WithTimeout(ctx, bc.config.Timeout)
	}
	defer cancel()
	var referenceClient *http.Client
	if bc.config != nil {
		referenceClient = bc.config.ImageReferenceHTTPClient
	}
	return downloadImageEditReference(downloadCtx, imageURL, referenceClient)
}

func downloadImageEditReference(ctx context.Context, imageURL string, override *http.Client) ([]byte, string, error) {
	validatedURL := strings.TrimSpace(imageURL)
	client := override
	if client == nil {
		var err error
		validatedURL, err = safeimagehttp.ValidatePublicHTTPSURL(validatedURL)
		if err != nil {
			return nil, "", fmt.Errorf("validate secondary image URL: %w", err)
		}
		client = safeimagehttp.NewPublicImageHTTPClient()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build secondary image request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("download secondary image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("download secondary image returned status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageReferenceBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read secondary image: %w", err)
	}
	if int64(len(data)) > maxImageReferenceBytes {
		return nil, "", fmt.Errorf("secondary image exceeds 32 MiB")
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("secondary image is empty")
	}
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return nil, "", fmt.Errorf("secondary image content type %q is not an image", contentType)
	}
	return data, contentType, nil
}

func imageEditFilename(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return "image.jpg"
	case "image/webp":
		return "image.webp"
	case "image/gif":
		return "image.gif"
	default:
		return "image.png"
	}
}

func (bc *BaseClient) doJSONImageRequest(ctx context.Context, method string, apiPath string, payload any) (*ImageResponse, error) {
	var lastErr error
	var lastResp *ImageResponse
	for attempt := 0; attempt <= bc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := bc.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			}
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, bc.config.Timeout)
		err := func() error {
			defer cancel()
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			request, err := http.NewRequestWithContext(timeoutCtx, method, buildAPIURL(bc.config.BaseURL, apiPath), bytes.NewReader(body))
			if err != nil {
				return err
			}
			request.Header.Set("Content-Type", "application/json")
			if bc.config.APIKey != "" {
				request.Header.Set("Authorization", "Bearer "+bc.config.APIKey)
			}
			resp, err := http.DefaultClient.Do(request)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				return fmt.Errorf("image api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
			}
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var parsed ImageResponse
			if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
				return err
			}
			parsed.RequestID = extractImageRequestID(resp.Header)
			parsed.RawResponse = strings.TrimSpace(string(bodyBytes))
			lastErr = nil
			payloadResp := parsed
			lastResp = &payloadResp
			return nil
		}()
		if err == nil {
			return lastResp, nil
		}
		lastErr = err
		if !shouldRetryWithContext(ctx, err) {
			break
		}
	}
	return nil, fmt.Errorf("调用 OpenAI image API 失败，已重试%d次: %w", bc.config.MaxRetries, lastErr)
}

func (bc *BaseClient) doMultipartImageRequest(ctx context.Context, apiPath string, body *bytes.Buffer, contentType string) (*ImageResponse, error) {
	var lastErr error
	var lastResp *ImageResponse
	for attempt := 0; attempt <= bc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := bc.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			}
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, bc.config.Timeout)
		err := func() error {
			defer cancel()
			request, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, buildAPIURL(bc.config.BaseURL, apiPath), bytes.NewReader(body.Bytes()))
			if err != nil {
				return err
			}
			request.Header.Set("Content-Type", contentType)
			if bc.config.APIKey != "" {
				request.Header.Set("Authorization", "Bearer "+bc.config.APIKey)
			}
			resp, err := http.DefaultClient.Do(request)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				return fmt.Errorf("image api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
			}
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var parsed ImageResponse
			if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
				return err
			}
			parsed.RequestID = extractImageRequestID(resp.Header)
			parsed.RawResponse = strings.TrimSpace(string(bodyBytes))
			lastErr = nil
			payloadResp := parsed
			lastResp = &payloadResp
			return nil
		}()
		if err == nil {
			return lastResp, nil
		}
		lastErr = err
		if !shouldRetryWithContext(ctx, err) {
			break
		}
	}
	return nil, fmt.Errorf("调用 OpenAI image edit API 失败，已重试%d次: %w", bc.config.MaxRetries, lastErr)
}

func buildAPIURL(baseURL string, apiPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return base + path.Clean("/"+apiPath)
}
