// Package models 提供TEMU平台图片相关数据结构定义
package models

// ImageValidationResult 图片验证结果
type ImageValidationResult struct {
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
	PaddedImage  []byte   `json:"-"` // 填充后的图片数据
	PaddedWidth  int      `json:"padded_width"`
	PaddedHeight int      `json:"padded_height"`
}

// ImageUploadRequest 图片上传请求
type ImageUploadRequest struct {
	ImageURL string `json:"image_url"`
	Type     string `json:"type"` // main, carousel, dimension
}

// ImageUploadResponse 图片上传响应
type ImageUploadResponse struct {
	Success   bool         `json:"success"`
	ErrorCode int          `json:"error_code"`
	Result    UploadResult `json:"result"`
}

// TemuImageUploadResponse Temu实际的图片上传响应格式
type TemuImageUploadResponse struct {
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
	URL      string `json:"url"` // 兼容字段
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

// ImageProcessingOptions 图片处理选项
type ImageProcessingOptions struct {
	Resize       bool   `json:"resize"`        // 是否调整尺寸
	Compress     bool   `json:"compress"`      // 是否压缩
	Convert      bool   `json:"convert"`       // 是否转换格式
	Quality      int    `json:"quality"`       // 压缩质量 (1-100)
	TargetWidth  int    `json:"target_width"`  // 目标宽度
	TargetHeight int    `json:"target_height"` // 目标高度
	TargetFormat string `json:"target_format"` // 目标格式
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
