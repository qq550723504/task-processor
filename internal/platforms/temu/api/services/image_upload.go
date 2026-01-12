// Package services 提供TEMU平台图片上传API功能
package services

import (
	"fmt"
	"task-processor/internal/pkg/downloader"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// ImageUploadAPI 图片上传API管理器
type ImageUploadAPI struct {
	client     client.APIClientInterface
	logger     *logrus.Entry
	downloader *downloader.ImageDownloader
}

// NewImageUploadAPI 创建新的图片上传API管理器
func NewImageUploadAPI(client client.APIClientInterface, logger *logrus.Entry) *ImageUploadAPI {
	return &ImageUploadAPI{
		client:     client,
		logger:     logger,
		downloader: downloader.NewImageDownloader(),
	}
}

// NeedsUpload 判断图片是否需要上传
func (i *ImageUploadAPI) NeedsUpload(imageURL string) bool {
	// 如果是TEMU的图片URL，不需要重新上传
	if imageURL == "" {
		return false
	}

	// 检查是否已经是TEMU的图片URL
	// TEMU图片URL通常包含特定的域名模式
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
		if len(imageURL) > len(domain) && imageURL[8:8+len(domain)] == domain {
			return false
		}
	}

	return true
}

// DownloadImage 下载图片
func (i *ImageUploadAPI) DownloadImage(imageURL string) ([]byte, string, error) {
	return i.downloader.DownloadImage(imageURL)
}

// GetUploadSignature 获取上传签名
func (i *ImageUploadAPI) GetUploadSignature() (*models.UploadSignature, error) {
	// 构造请求体
	requestBody := map[string]any{
		"upload_file_type": 1,
	}

	// 构造获取签名的API请求
	request := map[string]any{
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
	response := &models.SignatureResponse{}
	err := i.client.SendTEMURequest(request, response)
	if err != nil {
		i.logger.WithError(err).Error("发送获取签名请求失败")
		return nil, fmt.Errorf("发送获取签名请求失败: %w", err)
	}

	// 检查响应结果
	if !response.Success {
		i.logger.Errorf("获取签名失败: error_code=%d", response.ErrorCode)
		return nil, fmt.Errorf("获取签名失败: error_code=%d", response.ErrorCode)
	}

	i.logger.Info("成功获取上传签名")
	return &response.Result, nil
}

// UploadImageWithSignature 使用签名上传图片数据
func (i *ImageUploadAPI) UploadImageWithSignature(imageData []byte, filename string, signature *models.UploadSignature) (*models.UploadResult, error) {
	i.logger.Infof("开始上传图片: filename=%s, size=%d bytes", filename, len(imageData))

	// 构造API请求 - 使用fileFields和formFields
	request := map[string]any{
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
	response := &models.TemuImageUploadResponse{}
	err := i.client.SendTEMURequest(request, response)
	if err != nil {
		i.logger.WithError(err).Error("发送图片上传请求失败")
		return nil, fmt.Errorf("发送图片上传请求失败: %w", err)
	}

	// 检查响应
	if response.URL == "" {
		i.logger.Error("图片上传失败: 响应中没有URL")
		return nil, fmt.Errorf("图片上传失败: 响应中没有URL")
	}

	// 构造返回结果
	result := &models.UploadResult{
		ImageURL: response.URL,
		URL:      response.URL,
		Width:    response.Width,
		Height:   response.Height,
		Size:     response.Size,
		Format:   "jpg",
	}

	i.logger.Infof("图片上传成功: URL=%s, 尺寸=%dx%d", result.URL, result.Width, result.Height)
	return result, nil
}

// UploadImage 完整的图片上传流程（从URL到上传完成）
func (i *ImageUploadAPI) UploadImage(imageURL string) (*models.UploadResult, error) {
	// 检查是否需要上传
	if !i.NeedsUpload(imageURL) {
		i.logger.Infof("图片无需上传: %s", imageURL)
		return &models.UploadResult{
			ImageURL: imageURL,
			URL:      imageURL,
		}, nil
	}

	// 下载图片
	imageData, filename, err := i.DownloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	// 获取上传签名
	signature, err := i.GetUploadSignature()
	if err != nil {
		return nil, fmt.Errorf("获取上传签名失败: %w", err)
	}

	// 上传图片
	result, err := i.UploadImageWithSignature(imageData, filename, signature)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}

	return result, nil
}
