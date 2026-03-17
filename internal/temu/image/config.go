// Package image 提供TEMU平台图片处理相关功能
package image

import (
	"strings"
	"task-processor/internal/temu/handlerbase"
)

// ImageRequirement 图片要求配置（类型别名，来自 handlers/common）
type ImageRequirement = handlerbase.ImageRequirement

// imageConfig TEMU图片配置
type imageConfig struct {
	MaxImageCount     int
	MinWidth          int
	MinHeight         int
	MaxSize           int64
	MinAspectRatio    float64
	SupportedFormats  []string
	RecommendedWidth  int
	RecommendedHeight int
	RecommendedSize   int64
	CDNDomains        []string
}

var defaultImageConfig = &imageConfig{
	MaxImageCount:     49,
	MinWidth:          480,
	MinHeight:         480,
	MaxSize:           3 * 1024 * 1024,
	MinAspectRatio:    1.0 / 3.0,
	SupportedFormats:  []string{"JPEG", "JPG", "PNG"},
	RecommendedWidth:  1500,
	RecommendedHeight: 1500,
	RecommendedSize:   1 * 1024 * 1024,
	CDNDomains: []string{
		"img.kwcdn.com",
		"local-goods-image",
		"temu.com",
		"pdd.com",
	},
}

// isCDNImage 检查图片是否已经在TEMU CDN上
func isCDNImage(imageURL string) bool {
	for _, domain := range defaultImageConfig.CDNDomains {
		if strings.Contains(imageURL, domain) {
			return true
		}
	}
	return false
}

// needsUpload 判断图片是否需要上传
func needsUpload(imageURL string) bool {
	return !isCDNImage(imageURL)
}

// getImageRequirement 根据产品分类获取图片要求
func getImageRequirement(isClothes bool) ImageRequirement {
	if isClothes {
		return ImageRequirement{
			MaxSizeMB:     3.0,
			MinWidth:      1340,
			MinHeight:     1785,
			AspectRatio:   0.75,
			MinImageCount: 1,
			MaxImageCount: 10,
		}
	}
	return ImageRequirement{
		MaxSizeMB:     3.0,
		MinWidth:      800,
		MinHeight:     800,
		AspectRatio:   1.0,
		MinImageCount: 1,
		MaxImageCount: 10,
	}
}
