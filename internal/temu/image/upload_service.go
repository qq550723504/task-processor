// Package image 提供TEMU平台图片上传服务函数
package image

import (
	"fmt"
	"task-processor/internal/pkg/downloader"
	temuimage "task-processor/internal/temu/api/image"
)

// TemuAPIClient TEMU API客户端接口（避免循环导入）
type TemuAPIClient interface {
	SendTEMURequest(apiReq map[string]any, response any) error
}

// getUploadSignature 获取上传签名
func getUploadSignature(apiClient TemuAPIClient) (*temuimage.UploadSignature, error) {
	requestBody := map[string]any{
		"upload_file_type": 1,
	}
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

	response := &temuimage.SignatureResponse{}
	if err := apiClient.SendTEMURequest(apiReq, response); err != nil {
		return nil, fmt.Errorf("发送获取签名请求失败: %w", err)
	}
	if !response.Success {
		return nil, fmt.Errorf("获取签名失败: error_code=%d", response.ErrorCode)
	}
	return &response.Result, nil
}

// uploadImageWithSignature 使用签名上传图片数据
func uploadImageWithSignature(apiClient TemuAPIClient, imageData []byte, filename string, signature *temuimage.UploadSignature) (*temuimage.UploadResult, error) {
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

	response := &temuimage.TemuUploadResponse{}
	if err := apiClient.SendTEMURequest(apiReq, response); err != nil {
		return nil, fmt.Errorf("发送图片上传请求失败: %w", err)
	}
	if response.URL == "" {
		return nil, fmt.Errorf("图片上传失败: 响应中没有URL")
	}

	return &temuimage.UploadResult{
		ImageURL: response.URL,
		URL:      response.URL,
		Width:    response.Width,
		Height:   response.Height,
		Size:     response.Size,
		Format:   "jpg",
	}, nil
}

// downloadImage 下载图片
func downloadImage(imageURL string) ([]byte, string, error) {
	dl := downloader.NewImageDownloader()
	return dl.DownloadImage(imageURL)
}
