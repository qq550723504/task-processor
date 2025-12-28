// Package services 提供TEMU平台图片配置统一管理服务
package services

import (
	"strings"
)

// ImageConfigService 图片配置服务
type ImageConfigService struct {
	config *ImageConfig
}

// ImageConfig TEMU图片配置（统一配置）
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

	// CDN配置（统一管理）
	CDNDomains []string `json:"cdn_domains"` // TEMU CDN域名列表
}

// ImageQualityLevel 图片质量等级
type ImageQualityLevel int

const (
	ImageQualityLow    ImageQualityLevel = 1 // 低质量
	ImageQualityMedium ImageQualityLevel = 2 // 中等质量
	ImageQualityHigh   ImageQualityLevel = 3 // 高质量
)

// ImageRequirement 图片要求配置（统一定义）
type ImageRequirement struct {
	MaxSizeMB     float64 // 最大文件大小（MB）
	MinWidth      int     // 最小宽度
	MinHeight     int     // 最小高度
	AspectRatio   float64 // 期望宽高比（严格要求，不允许偏差）
	MinImageCount int     // 最小图片数量
	MaxImageCount int     // 最大图片数量
}

// NewImageConfigService 创建新的图片配置服务
func NewImageConfigService() *ImageConfigService {
	return &ImageConfigService{
		config: getDefaultImageConfig(),
	}
}

// getDefaultImageConfig 获取默认图片配置
func getDefaultImageConfig() *ImageConfig {
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

		// TEMU CDN域名（统一管理，避免重复）
		CDNDomains: []string{
			"img.kwcdn.com",
			"local-goods-image",
			"temu.com",
			"pdd.com",
		},
	}
}

// GetConfig 获取配置
func (s *ImageConfigService) GetConfig() *ImageConfig {
	return s.config
}

// GetImageRequirement 根据产品分类获取图片要求（统一逻辑）
func (s *ImageConfigService) GetImageRequirement(isClothes bool) ImageRequirement {
	if isClothes {
		// 服装类产品要求
		return ImageRequirement{
			MaxSizeMB:     3.0,
			MinWidth:      1340,
			MinHeight:     1785,
			AspectRatio:   0.75, // 3:4 = 0.75 (严格要求)
			MinImageCount: 1,
			MaxImageCount: 10,
		}
	}

	// 其他分类产品要求
	return ImageRequirement{
		MaxSizeMB:     3.0,
		MinWidth:      800,
		MinHeight:     800,
		AspectRatio:   1.0, // 1:1 (严格要求)
		MinImageCount: 1,
		MaxImageCount: 10,
	}
}

// IsCDNImage 检查图片是否已经在TEMU CDN上（统一逻辑）
func (s *ImageConfigService) IsCDNImage(imageURL string) bool {
	for _, domain := range s.config.CDNDomains {
		if strings.Contains(imageURL, domain) {
			return true
		}
	}
	return false
}

// NeedsUpload 判断图片是否需要上传（统一逻辑）
func (s *ImageConfigService) NeedsUpload(imageURL string) bool {
	return !s.IsCDNImage(imageURL)
}

// IsValidFormat 检查图片格式是否支持
func (s *ImageConfigService) IsValidFormat(format string) bool {
	for _, supportedFormat := range s.config.SupportedFormats {
		if format == supportedFormat {
			return true
		}
	}
	return false
}

// GetImageQualityLevel 根据图片参数获取质量等级
func (s *ImageConfigService) GetImageQualityLevel(width, height int, size int64) ImageQualityLevel {
	// 高质量：尺寸大于推荐值且文件大小适中
	if width >= s.config.RecommendedWidth && height >= s.config.RecommendedHeight &&
		size <= s.config.RecommendedSize {
		return ImageQualityHigh
	}

	// 中等质量：满足最低要求且尺寸合理
	if width >= s.config.MinWidth && height >= s.config.MinHeight &&
		size <= s.config.MaxSize && width >= 800 && height >= 800 {
		return ImageQualityMedium
	}

	// 低质量：仅满足最低要求
	return ImageQualityLow
}

// IsValidImage 检查图片是否符合基本要求
func (s *ImageConfigService) IsValidImage(width, height int, size int64, format string) bool {
	// 检查尺寸
	if width < s.config.MinWidth || height < s.config.MinHeight {
		return false
	}

	// 检查文件大小
	if size > s.config.MaxSize {
		return false
	}

	// 检查宽高比
	aspectRatio := float64(width) / float64(height)
	if aspectRatio < s.config.MinAspectRatio {
		return false
	}

	// 检查格式
	return s.IsValidFormat(format)
}
