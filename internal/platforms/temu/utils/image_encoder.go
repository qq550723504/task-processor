// Package utils 提供图片编码相关工具方法
package utils

import (
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
)

// ImageEncoder 图片编码器
type ImageEncoder struct {
	logger *logrus.Entry
}

// NewImageEncoder 创建新的图片编码器
func NewImageEncoder() *ImageEncoder {
	return &ImageEncoder{
		logger: logrus.WithField("component", "ImageEncoder"),
	}
}

// EncodeImage 编码图片到指定格式
func (e *ImageEncoder) EncodeImage(w io.Writer, img image.Image, format string) error {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 95})
	case "png":
		return png.Encode(w, img)
	default:
		// 默认使用JPEG
		e.logger.Warnf("未知格式 %s，使用默认JPEG格式", format)
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 95})
	}
}

// GetOptimalFormat 根据图片特征获取最优格式
func (e *ImageEncoder) GetOptimalFormat(img image.Image, originalFormat string) string {
	// 如果原格式是PNG且图片可能有透明度，保持PNG
	if strings.ToLower(originalFormat) == "png" {
		if e.hasTransparency(img) {
			return "png"
		}
	}

	// 默认使用JPEG，压缩比更好
	return "jpeg"
}

// hasTransparency 检查图片是否有透明度
func (e *ImageEncoder) hasTransparency(img image.Image) bool {
	bounds := img.Bounds()

	// 采样检查，避免检查每个像素
	sampleSize := 10
	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleSize {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleSize {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 65535 { // 不是完全不透明
				return true
			}
		}
	}

	return false
}
