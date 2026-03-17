// Package api 提供Amazon SP-API图片上传功能
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

// UploadDestinationRequest 上传目标请求
type UploadDestinationRequest struct {
	Resource      string            `json:"resource"`
	MarketplaceID string            `json:"marketplaceId"`
	ContentMD5    string            `json:"contentMD5,omitempty"`
	ContentType   string            `json:"contentType,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// UploadDestinationResponse 上传目标响应
type UploadDestinationResponse struct {
	Payload struct {
		UploadDestinationID string            `json:"uploadDestinationId"`
		URL                 string            `json:"url"`
		Headers             map[string]string `json:"headers"`
	} `json:"payload"`
	Errors []APIError `json:"errors,omitempty"`
}

// ImageUploadResult 图片上传结果
type ImageUploadResult struct {
	ImageID     string `json:"imageId"`
	URL         string `json:"url"`
	OriginalURL string `json:"originalUrl"`
}

// CreateUploadDestination 创建上传目标
func (c *Client) CreateUploadDestination(ctx context.Context, resource, marketplaceID, contentType string) (*UploadDestinationResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"resource":     resource,
		"marketplace":  marketplaceID,
		"content_type": contentType,
	}).Info("创建图片上传目标")

	req := UploadDestinationRequest{
		Resource:      resource,
		MarketplaceID: marketplaceID,
		ContentType:   contentType,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/uploads/v1/uploadDestinations", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 获取访问令牌
	accessToken, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-amz-access-token", accessToken)
	httpReq.Header.Set("User-Agent", "task-processor/1.0")

	// 如果有AWS签名器，进行签名
	if c.awsSigner != nil {
		if signErr := c.awsSigner.SignRequest(httpReq, reqBody); signErr != nil {
			return nil, fmt.Errorf("AWS签名失败: %w", signErr)
		}
		c.logger.Debug("✅ AWS签名已应用")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var result UploadDestinationResponse
	if err := jsonx.UnmarshalBytes(body, &result, "解析响应失败"); err != nil {
		return nil, err
	}

	c.logger.WithField("upload_destination_id", result.Payload.UploadDestinationID).Info("上传目标创建成功")
	return &result, nil
}

// UploadImageToDestination 上传图片到指定目标
func (c *Client) UploadImageToDestination(ctx context.Context, destination *UploadDestinationResponse, imageData []byte, filename string) error {
	c.logger.WithFields(logrus.Fields{
		"destination_id": destination.Payload.UploadDestinationID,
		"filename":       filename,
		"size":           len(imageData),
	}).Info("上传图片到Amazon")

	// 创建multipart表单
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加文件字段
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("创建表单文件字段失败: %w", err)
	}

	if _, writeErr := part.Write(imageData); writeErr != nil {
		return fmt.Errorf("写入文件数据失败: %w", writeErr)
	}

	if closeErr := writer.Close(); closeErr != nil {
		return fmt.Errorf("关闭multipart writer失败: %w", closeErr)
	}

	// 创建上传请求
	req, err := http.NewRequestWithContext(ctx, "POST", destination.Payload.URL, &buf)
	if err != nil {
		return fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 添加Amazon要求的头部
	for key, value := range destination.Payload.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("上传图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("图片上传失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	c.logger.Info("图片上传成功")
	return nil
}

// UploadImage 完整的图片上传流程
func (c *Client) UploadImage(ctx context.Context, imageData []byte, filename, marketplaceID string) (*ImageUploadResult, error) {
	// 1. 确定内容类型
	contentType := "image/jpeg"
	ext := filepath.Ext(filename)
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	}

	// 2. 创建上传目标
	destination, err := c.CreateUploadDestination(ctx, "LISTING_IMAGE", marketplaceID, contentType)
	if err != nil {
		return nil, fmt.Errorf("创建上传目标失败: %w", err)
	}

	// 3. 上传图片
	if uploadErr := c.UploadImageToDestination(ctx, destination, imageData, filename); uploadErr != nil {
		return nil, fmt.Errorf("上传图片失败: %w", uploadErr)
	}

	// 4. 返回结果
	result := &ImageUploadResult{
		ImageID:     destination.Payload.UploadDestinationID,
		URL:         destination.Payload.URL,
		OriginalURL: "", // 原始URL在这里不适用
	}

	c.logger.WithField("image_id", result.ImageID).Info("图片上传完成")
	return result, nil
}
