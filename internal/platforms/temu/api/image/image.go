// Package image 提供TEMU平台图片上传和验证相关的API和数据结构
package image

import (
	"fmt"
	"task-processor/internal/pkg/downloader"
	"task-processor/internal/platforms/temu/api/client"

	"github.com/sirupsen/logrus"
)

// --- 数据模型 ---

// ValidationResult 图片验证结果
type ValidationResult struct {
	IsValid      bool     `json:"is_valid"`
	URL          string   `json:"url"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Format       string   `json:"format"`
	Size         int64    `json:"size"`
	AspectRatio  float64  `json:"aspect_ratio"`
	Violations   []string `json:"violations"`
	Suggestions  []string `json:"suggestions"`
	NeedsPadding bool     `json:"needs_padding"`
	PaddedImage  []byte   `json:"-"`
	PaddedWidth  int      `json:"padded_width"`
	PaddedHeight int      `json:"padded_height"`
}

// UploadRequest 图片上传请求
type UploadRequest struct {
	ImageURL string `json:"image_url"`
	Type     string `json:"type"`
}

// UploadResponse 图片上传响应
type UploadResponse struct {
	Success   bool         `json:"success"`
	ErrorCode int          `json:"error_code"`
	Result    UploadResult `json:"result"`
}

// TemuUploadResponse Temu实际的图片上传响应格式
type TemuUploadResponse struct {
	URL           string   `json:"url"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Size          int64    `json:"size"`
	ProcessedURLs []string `json:"processed_urls"`
	Etag          string   `json:"etag"`
}

// UploadResult 上传结果
type UploadResult struct {
	ImageURL string `json:"image_url"`
	ImageID  string `json:"image_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Size     int64  `json:"size"`
	Format   string `json:"format"`
	URL      string `json:"url"`
}

// SignatureResponse 签名响应
type SignatureResponse struct {
	Success      bool            `json:"success"`
	ErrorCode    int             `json:"error_code"`
	Result       UploadSignature `json:"result"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// UploadSignature 上传签名
type UploadSignature struct {
	Signature string `json:"signature"`
}

// ProcessingOptions 图片处理选项
type ProcessingOptions struct {
	Resize       bool   `json:"resize"`
	Compress     bool   `json:"compress"`
	Convert      bool   `json:"convert"`
	Quality      int    `json:"quality"`
	TargetWidth  int    `json:"target_width"`
	TargetHeight int    `json:"target_height"`
	TargetFormat string `json:"target_format"`
}

// PaddingResult 填充结果
type PaddingResult struct {
	Success      bool
	OriginalURL  string
	PaddedImage  []byte
	NewWidth     int
	NewHeight    int
	Format       string
	NeedsPadding bool
	Error        error
}

// --- API ---

// API 图片上传API管理器
type API struct {
	client     client.APIClientInterface
	logger     *logrus.Entry
	downloader *downloader.ImageDownloader
}

// NewAPI 创建图片上传API管理器
func NewAPI(c client.APIClientInterface, logger *logrus.Entry) *API {
	return &API{
		client:     c,
		logger:     logger,
		downloader: downloader.NewImageDownloader(),
	}
}

// NeedsUpload 判断图片是否需要上传
func (a *API) NeedsUpload(imageURL string) bool {
	if imageURL == "" {
		return false
	}
	temuDomains := []string{
		"img.kwcdn.com",
		"img.ltwebstatic.com",
		"img.alicdn.com",
		"ae01.alicdn.com",
		"ae02.alicdn.com",
		"ae03.alicdn.com",
		"ae04.alicdn.com",
	}
	for _, domain := range temuDomains {
		if len(imageURL) > 8+len(domain) && imageURL[8:8+len(domain)] == domain {
			return false
		}
	}
	return true
}

// DownloadImage 下载图片
func (a *API) DownloadImage(imageURL string) ([]byte, string, error) {
	return a.downloader.DownloadImage(imageURL)
}

// GetUploadSignature 获取上传签名
func (a *API) GetUploadSignature() (*UploadSignature, error) {
	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/get_signature",
		"headers": map[string]string{
			"accept":         "application/json, text/plain, */*",
			"content-type":   "application/json;charset=UTF-8",
			"sec-fetch-dest": "empty",
			"sec-fetch-mode": "cors",
			"sec-fetch-site": "same-origin",
		},
		"body": map[string]any{"upload_file_type": 1},
	}

	var resp SignatureResponse
	if err := a.client.SendTEMURequest(req, &resp); err != nil {
		return nil, fmt.Errorf("发送获取签名请求失败: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("获取签名失败: error_code=%d", resp.ErrorCode)
	}
	return &resp.Result, nil
}

// UploadWithSignature 使用签名上传图片数据
func (a *API) UploadWithSignature(imageData []byte, filename string, signature *UploadSignature) (*UploadResult, error) {
	req := map[string]any{
		"method": "POST",
		"url":    "/api/galerie/v3/store_image?sdk_version=js-1.0.6&tag_name=local-goods-image",
		"headers": map[string]string{
			"accept":         "*/*",
			"sec-fetch-dest": "empty",
			"sec-fetch-mode": "cors",
			"sec-fetch-site": "same-origin",
		},
		"formFields": map[string]string{
			"url_width_height": "true",
			"pic_operations":   `{"original_needed":false,"rules":[{"rule":"imageMogr2/format/jpg|imageMogr2/size-limit/3m!/ignore-error/0","suffix":"format"}]}`,
			"upload_sign":      signature.Signature,
		},
		"fileFields": map[string]any{
			"image": map[string]any{
				"filename": filename,
				"content":  imageData,
			},
		},
	}

	var resp TemuUploadResponse
	if err := a.client.SendTEMURequest(req, &resp); err != nil {
		return nil, fmt.Errorf("发送图片上传请求失败: %w", err)
	}
	if resp.URL == "" {
		return nil, fmt.Errorf("图片上传失败: 响应中没有URL")
	}
	return &UploadResult{
		ImageURL: resp.URL,
		URL:      resp.URL,
		Width:    resp.Width,
		Height:   resp.Height,
		Size:     resp.Size,
		Format:   "jpg",
	}, nil
}

// Upload 完整的图片上传流程（从URL到上传完成）
func (a *API) Upload(imageURL string) (*UploadResult, error) {
	if !a.NeedsUpload(imageURL) {
		return &UploadResult{ImageURL: imageURL, URL: imageURL}, nil
	}

	imageData, filename, err := a.DownloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	signature, err := a.GetUploadSignature()
	if err != nil {
		return nil, fmt.Errorf("获取上传签名失败: %w", err)
	}

	result, err := a.UploadWithSignature(imageData, filename, signature)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}
	return result, nil
}
