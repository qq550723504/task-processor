package service

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
)

// ImageProcessor 图片处理器
type ImageProcessor struct {
	logger *logrus.Entry
}

// NewImageProcessor 创建图片处理器
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		logger: logrus.WithField("service", "ImageProcessor"),
	}
}

// Resize 调整图片大小
func (p *ImageProcessor) Resize(imageData []byte, width, height int) ([]byte, error) {
	p.logger.Infof("调整图片大小: %dx%d", width, height)

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	// 调整大小（保持宽高比）
	resized := imaging.Fit(img, width, height, imaging.Lanczos)

	// 编码图片
	var buf bytes.Buffer
	switch format {
	case "jpeg", "jpg":
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("编码JPEG失败: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, resized); err != nil {
			return nil, fmt.Errorf("编码PNG失败: %w", err)
		}
	default:
		// 默认使用JPEG
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("编码图片失败: %w", err)
		}
	}

	p.logger.Infof("图片处理完成，大小: %d bytes", buf.Len())
	return buf.Bytes(), nil
}

// ValidateFormat 验证图片格式
func (p *ImageProcessor) ValidateFormat(imageData []byte) error {
	_, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return fmt.Errorf("无效的图片格式: %w", err)
	}

	// Amazon 支持的图片格式
	allowedFormats := map[string]bool{
		"jpeg": true,
		"jpg":  true,
		"png":  true,
		"gif":  true,
	}

	if !allowedFormats[format] {
		return fmt.Errorf("不支持的图片格式: %s", format)
	}

	return nil
}

// GetImageInfo 获取图片信息
func (p *ImageProcessor) GetImageInfo(imageData []byte) (*ImageInfo, error) {
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	bounds := img.Bounds()
	return &ImageInfo{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Format: format,
		Size:   len(imageData),
	}, nil
}

// ImageInfo 图片信息
type ImageInfo struct {
	Width  int
	Height int
	Format string
	Size   int
}
