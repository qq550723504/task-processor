package handlers

import (
	"fmt"
	"strings"
)

// ImageConfig TEMU图片配置
type ImageConfig struct {
	// 基本限制
	MaxImageCount  int     `json:"max_image_count"`  // 最大图片数量
	MinWidth       int     `json:"min_width"`        // 最小宽度
	MinHeight      int     `json:"min_height"`       // 最小高度
	MaxSize        int64   `json:"max_size"`         // 最大文件大小（字节）
	MinAspectRatio float64 `json:"min_aspect_ratio"` // 最小宽高比

	// 支持的格式
	SupportedFormats []string `json:"supported_formats"` // 支持的图片格式

	// 推荐参数
	RecommendedWidth  int   `json:"recommended_width"`  // 推荐宽度
	RecommendedHeight int   `json:"recommended_height"` // 推荐高度
	RecommendedSize   int64 `json:"recommended_size"`   // 推荐文件大小

	// API配置
	UploadAPI     string `json:"upload_api"`     // 上传API端点
	ConversionAPI string `json:"conversion_api"` // 转换API端点

	// CDN配置
	CDNDomains []string `json:"cdn_domains"` // TEMU CDN域名列表
}

// GetDefaultImageConfig 获取默认图片配置
func GetDefaultImageConfig() *ImageConfig {
	return &ImageConfig{
		// TEMU官方限制
		MaxImageCount:  49,
		MinWidth:       480,
		MinHeight:      480,
		MaxSize:        3 * 1024 * 1024, // 3MB
		MinAspectRatio: 1.0 / 3.0,       // 1:3

		// 支持的格式
		SupportedFormats: []string{"JPEG", "JPG", "PNG"},

		// 推荐参数（更好的显示效果）
		RecommendedWidth:  1500,
		RecommendedHeight: 1500,
		RecommendedSize:   1 * 1024 * 1024, // 1MB

		// API端点
		UploadAPI:     "/bg/local/goods/image/upload",
		ConversionAPI: "/bg/local/goods/image/convert",

		// TEMU CDN域名
		CDNDomains: []string{
			"img.kwcdn.com",
			"local-goods-image",
			"temu.com",
			"pdd.com",
		},
	}
}

// ImageQualityLevel 图片质量等级
type ImageQualityLevel int

const (
	ImageQualityLow    ImageQualityLevel = 1 // 低质量
	ImageQualityMedium ImageQualityLevel = 2 // 中等质量
	ImageQualityHigh   ImageQualityLevel = 3 // 高质量
)

// GetImageQualityLevel 根据图片参数获取质量等级
func (config *ImageConfig) GetImageQualityLevel(width, height int, size int64) ImageQualityLevel {
	// 高质量：尺寸大于推荐值且文件大小适中
	if width >= config.RecommendedWidth && height >= config.RecommendedHeight &&
		size <= config.RecommendedSize {
		return ImageQualityHigh
	}

	// 中等质量：满足最低要求且尺寸合理
	if width >= config.MinWidth && height >= config.MinHeight &&
		size <= config.MaxSize && width >= 800 && height >= 800 {
		return ImageQualityMedium
	}

	// 低质量：仅满足最低要求
	return ImageQualityLow
}

// IsValidImage 检查图片是否符合基本要求
func (config *ImageConfig) IsValidImage(width, height int, size int64, format string) bool {
	// 检查尺寸
	if width < config.MinWidth || height < config.MinHeight {
		return false
	}

	// 检查文件大小
	if size > config.MaxSize {
		return false
	}

	// 检查宽高比
	aspectRatio := float64(width) / float64(height)
	if aspectRatio < config.MinAspectRatio {
		return false
	}

	// 检查格式
	for _, supportedFormat := range config.SupportedFormats {
		if format == supportedFormat {
			return true
		}
	}

	return false
}

// GetOptimizationSuggestions 获取图片优化建议
func (config *ImageConfig) GetOptimizationSuggestions(width, height int, size int64, format string) []string {
	var suggestions []string

	// 尺寸建议
	if width < config.RecommendedWidth || height < config.RecommendedHeight {
		suggestions = append(suggestions,
			fmt.Sprintf("建议使用更高分辨率的图片 (推荐: %dx%d)",
				config.RecommendedWidth, config.RecommendedHeight))
	}

	// 文件大小建议
	if size > config.RecommendedSize {
		suggestions = append(suggestions,
			fmt.Sprintf("建议压缩图片文件大小 (推荐: <%.1fMB)",
				float64(config.RecommendedSize)/(1024*1024)))
	}

	// 宽高比建议
	aspectRatio := float64(width) / float64(height)
	if aspectRatio < 0.5 {
		suggestions = append(suggestions, "图片过于狭长，建议使用更接近正方形的图片")
	}

	// 格式建议
	isValidFormat := false
	for _, supportedFormat := range config.SupportedFormats {
		if format == supportedFormat {
			isValidFormat = true
			break
		}
	}
	if !isValidFormat {
		suggestions = append(suggestions,
			fmt.Sprintf("不支持的图片格式 %s，请使用 %v", format, config.SupportedFormats))
	}

	return suggestions
}

// IsCDNImage 检查图片是否已经在TEMU CDN上
func (config *ImageConfig) IsCDNImage(imageURL string) bool {
	for _, domain := range config.CDNDomains {
		if strings.Contains(imageURL, domain) {
			return true
		}
	}
	return false
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

// GetDefaultProcessingOptions 获取默认处理选项
func GetDefaultProcessingOptions() *ImageProcessingOptions {
	return &ImageProcessingOptions{
		Resize:       false,
		Compress:     true,
		Convert:      false,
		Quality:      85,
		TargetWidth:  1500,
		TargetHeight: 1500,
		TargetFormat: "JPEG",
	}
}
