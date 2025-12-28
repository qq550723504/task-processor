// Package services 提供TEMU平台图片上传统一服务
package services

import (
	"fmt"
	"task-processor/internal/common/downloader"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// TemuAPIClient TEMU API客户端接口（避免循环导入）
type TemuAPIClient interface {
	SendTEMURequest(apiReq map[string]any, response any) error
}

// ImageUploadService 图片上传服务（统一管理上传逻辑）
type ImageUploadService struct {
	logger        *logrus.Entry
	configService *ImageConfigService
	downloader    *downloader.ImageDownloader
}

// NewImageUploadService 创建新的图片上传服务
func NewImageUploadService() *ImageUploadService {
	return &ImageUploadService{
		logger:        logrus.WithField("service", "ImageUploadService"),
		configService: NewImageConfigService(),
		downloader:    downloader.NewImageDownloader(),
	}
}

// NeedsUpload 判断图片是否需要上传（统一逻辑）
func (s *ImageUploadService) NeedsUpload(imageURL string) bool {
	return s.configService.NeedsUpload(imageURL)
}

// DownloadImage 下载图片（使用现有的下载器）
func (s *ImageUploadService) DownloadImage(imageURL string) ([]byte, string, error) {
	return s.downloader.DownloadImage(imageURL)
}

// GetUploadSignature 获取上传签名（统一实现）
func (s *ImageUploadService) GetUploadSignature(apiClient TemuAPIClient) (*types.UploadSignature, error) {
	// 构造请求体
	requestBody := map[string]any{
		"upload_file_type": 1,
	}

	// 构造获取签名的API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/get_signature",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": requestBody,
	}

	// 发送请求获取签名
	response := &types.SignatureResponse{}
	err := apiClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return nil, fmt.Errorf("发送获取签名请求失败: %w", err)
	}

	// 检查响应结果
	if !response.Success {
		return nil, fmt.Errorf("获取签名失败: error_code=%d", response.ErrorCode)
	}

	return &response.Result, nil
}

// UploadImageWithSignature 使用签名上传图片数据（统一实现）
func (s *ImageUploadService) UploadImageWithSignature(apiClient TemuAPIClient, imageData []byte, filename string, signature *types.UploadSignature) (*types.UploadResult, error) {
	// 构造API请求 - 使用fileFields和formFields
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/api/galerie/v3/store_image?sdk_version=js-1.0.6&tag_name=local-goods-image",
		"headers": map[string]string{
			"accept":             "*/*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
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

	// 发送上传请求
	response := &types.TemuImageUploadResponse{}
	err := apiClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return nil, fmt.Errorf("发送图片上传请求失败: %w", err)
	}

	// 检查响应
	if response.URL == "" {
		return nil, fmt.Errorf("图片上传失败: 响应中没有URL")
	}

	// 构造返回结果
	result := &types.UploadResult{
		ImageURL: response.URL,
		URL:      response.URL,
		Width:    response.Width,
		Height:   response.Height,
		Size:     response.Size,
		Format:   "jpg",
	}

	return result, nil
}
