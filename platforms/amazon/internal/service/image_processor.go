// Package service 提供图片处理功能
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

// ImageInfo 图片信息
type ImageInfo struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Size   int    `json:"size"`
}

// ProcessingOptions 图片处理选项
type ProcessingOptions struct {
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	Quality int  `json:"quality"`
	Fit     bool `json:"fit"` // 是否保持宽高比
}

// Resize 调整图片大小
func (p *ImageProcessor) Resize(imageData []byte, width, height int) ([]byte, error) {
	p.logger.WithFields(logrus.Fields{
		"target_width":  width,
		"target_height": height,
		"original_size": len(imageData),
	}).Info("开始调整图片大小")

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	// 调整大小（保持宽高比）
	resized := imaging.Fit(img, width, height, imaging.Lanczos)

	// 编码图片
	result, err := p.encodeImage(resized, format)
	if err != nil {
		return nil, err
	}

	p.logger.WithFields(logrus.Fields{
		"original_size":  len(imageData),
		"processed_size": len(result),
		"format":         format,
	}).Info("图片大小调整完成")

	return result, nil
}

// ResizeWithOptions 使用选项调整图片大小
func (p *ImageProcessor) ResizeWithOptions(imageData []byte, options ProcessingOptions) ([]byte, error) {
	p.logger.WithFields(logrus.Fields{
		"options":       options,
		"original_size": len(imageData),
	}).Info("开始处理图片")

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	var processed image.Image
	if options.Fit {
		// 保持宽高比
		processed = imaging.Fit(img, options.Width, options.Height, imaging.Lanczos)
	} else {
		// 强制调整到指定尺寸
		processed = imaging.Resize(img, options.Width, options.Height, imaging.Lanczos)
	}

	// 编码图片
	result, err := p.encodeImageWithQuality(processed, format, options.Quality)
	if err != nil {
		return nil, err
	}

	p.logger.WithFields(logrus.Fields{
		"original_size":     len(imageData),
		"processed_size":    len(result),
		"compression_ratio": float64(len(result)) / float64(len(imageData)),
	}).Info("图片处理完成")

	return result, nil
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
		return fmt.Errorf("不支持的图片格式: %s，支持的格式: jpeg, png, gif", format)
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

// ValidateSize 验证图片尺寸
func (p *ImageProcessor) ValidateSize(imageData []byte, maxWidth, maxHeight, maxSize int) error {
	info, err := p.GetImageInfo(imageData)
	if err != nil {
		return err
	}

	if info.Size > maxSize {
		return fmt.Errorf("图片文件过大: %d bytes，最大允许: %d bytes", info.Size, maxSize)
	}

	if info.Width > maxWidth {
		return fmt.Errorf("图片宽度过大: %d pixels，最大允许: %d pixels", info.Width, maxWidth)
	}

	if info.Height > maxHeight {
		return fmt.Errorf("图片高度过大: %d pixels，最大允许: %d pixels", info.Height, maxHeight)
	}

	return nil
}

// OptimizeForAmazon 为Amazon优化图片
func (p *ImageProcessor) OptimizeForAmazon(imageData []byte) ([]byte, error) {
	p.logger.Info("为Amazon平台优化图片")

	// Amazon推荐的图片规格
	const (
		maxWidth  = 2560
		maxHeight = 2560
		minWidth  = 1000
		minHeight = 1000
		quality   = 85
	)

	info, err := p.GetImageInfo(imageData)
	if err != nil {
		return nil, err
	}

	// 如果图片已经符合要求，直接返回
	if info.Width >= minWidth && info.Width <= maxWidth &&
		info.Height >= minHeight && info.Height <= maxHeight {
		return imageData, nil
	}

	// 计算目标尺寸
	targetWidth := info.Width
	targetHeight := info.Height

	// 如果太小，放大到最小尺寸
	if info.Width < minWidth || info.Height < minHeight {
		scale := float64(minWidth) / float64(info.Width)
		if float64(minHeight)/float64(info.Height) > scale {
			scale = float64(minHeight) / float64(info.Height)
		}
		targetWidth = int(float64(info.Width) * scale)
		targetHeight = int(float64(info.Height) * scale)
	}

	// 如果太大，缩小到最大尺寸
	if targetWidth > maxWidth || targetHeight > maxHeight {
		scale := float64(maxWidth) / float64(targetWidth)
		if float64(maxHeight)/float64(targetHeight) < scale {
			scale = float64(maxHeight) / float64(targetHeight)
		}
		targetWidth = int(float64(targetWidth) * scale)
		targetHeight = int(float64(targetHeight) * scale)
	}

	// 处理图片
	options := ProcessingOptions{
		Width:   targetWidth,
		Height:  targetHeight,
		Quality: quality,
		Fit:     true,
	}

	result, err := p.ResizeWithOptions(imageData, options)
	if err != nil {
		return nil, fmt.Errorf("优化图片失败: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"original_size":       fmt.Sprintf("%dx%d", info.Width, info.Height),
		"optimized_size":      fmt.Sprintf("%dx%d", targetWidth, targetHeight),
		"file_size_reduction": fmt.Sprintf("%.1f%%", (1.0-float64(len(result))/float64(len(imageData)))*100),
	}).Info("Amazon图片优化完成")

	return result, nil
}

// BatchProcess 批量处理图片
func (p *ImageProcessor) BatchProcess(images [][]byte, options ProcessingOptions) ([][]byte, error) {
	p.logger.WithField("count", len(images)).Info("开始批量处理图片")

	results := make([][]byte, 0, len(images))
	var errors []error

	for i, imageData := range images {
		processed, err := p.ResizeWithOptions(imageData, options)
		if err != nil {
			p.logger.WithError(err).Warnf("处理图片 %d 失败", i+1)
			errors = append(errors, err)
			continue
		}
		results = append(results, processed)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("所有图片处理失败，第一个错误: %w", errors[0])
	}

	p.logger.WithFields(logrus.Fields{
		"success_count": len(results),
		"total_count":   len(images),
		"error_count":   len(errors),
	}).Info("批量图片处理完成")

	return results, nil
}

// encodeImage 编码图片
func (p *ImageProcessor) encodeImage(img image.Image, format string) ([]byte, error) {
	return p.encodeImageWithQuality(img, format, 95)
}

// encodeImageWithQuality 使用指定质量编码图片
func (p *ImageProcessor) encodeImageWithQuality(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer

	switch format {
	case "jpeg", "jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("编码JPEG失败: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("编码PNG失败: %w", err)
		}
	default:
		// 默认使用JPEG
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("编码图片失败: %w", err)
		}
	}

	return buf.Bytes(), nil
}
