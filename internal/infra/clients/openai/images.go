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
	"time"
)

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
	if len(req.Image) == 0 {
		return nil, fmt.Errorf("image edit request requires image bytes")
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
	imagePart, err := writer.CreateFormFile("image", "image.png")
	if err != nil {
		return nil, fmt.Errorf("create image form file: %w", err)
	}
	if _, err := imagePart.Write(req.Image); err != nil {
		return nil, fmt.Errorf("write image form file: %w", err)
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
