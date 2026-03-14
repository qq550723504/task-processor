package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"task-processor/internal/pkg/downloader"
	pkgutils "task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// ImagePaddingProcessor 图片填充处理器
type ImagePaddingProcessor struct {
	logger     *logrus.Entry
	downloader *downloader.ImageDownloader
}

// NewImagePaddingProcessor 创建新的图片填充处理器
func NewImagePaddingProcessor() *ImagePaddingProcessor {
	return &ImagePaddingProcessor{
		logger:     logrus.WithField("processor", "ImagePaddingProcessor"),
		downloader: downloader.NewImageDownloader(),
	}
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

// PadImageToAspectRatio 将图片填充到指定宽高比
func (p *ImagePaddingProcessor) PadImageToAspectRatio(imageURL string, targetRatio float64, minWidth, minHeight int) (*PaddingResult, error) {
	result := &PaddingResult{
		OriginalURL: imageURL,
		Success:     false,
	}

	// 下载图片
	img, format, err := p.downloadImage(imageURL)
	if err != nil {
		result.Error = fmt.Errorf("下载图片失败: %w", err)
		return result, result.Error
	}

	result.Format = format
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()
	currentRatio := float64(originalWidth) / float64(originalHeight)

	// 检查是否需要填充 - 严格匹配，不允许容差
	// 只有当宽高比完全匹配且尺寸满足要求时才跳过填充
	if currentRatio == targetRatio && originalWidth >= minWidth && originalHeight >= minHeight {
		result.NeedsPadding = false
		result.Success = true
		result.NewWidth = originalWidth
		result.NewHeight = originalHeight
		return result, nil
	}

	result.NeedsPadding = true

	// 计算新的尺寸
	var newWidth, newHeight int
	if currentRatio > targetRatio {
		// 图片太宽，需要在上下添加白边
		newWidth = originalWidth
		newHeight = int(float64(originalWidth) / targetRatio)
	} else {
		// 图片太高，需要在左右添加白边
		newHeight = originalHeight
		newWidth = int(float64(originalHeight) * targetRatio)
	}

	// 对于1:1比例，确保宽高完全相等
	if targetRatio == 1.0 {
		maxDimension := newWidth
		if newHeight > maxDimension {
			maxDimension = newHeight
		}
		newWidth = maxDimension
		newHeight = maxDimension
		p.logger.Infof("🔧 强制1:1比例: %dx%d -> %dx%d", originalWidth, originalHeight, newWidth, newHeight)
	}

	// 确保满足最小尺寸要求
	if newWidth < minWidth || newHeight < minHeight {
		// 对于1:1比例，使用最大的最小尺寸要求
		if targetRatio == 1.0 {
			requiredSize := minWidth
			if minHeight > requiredSize {
				requiredSize = minHeight
			}
			newWidth = requiredSize
			newHeight = requiredSize
			p.logger.Infof("🔧 1:1比例最小尺寸调整: %dx%d -> %dx%d", originalWidth, originalHeight, newWidth, newHeight)
		} else {
			// 非1:1比例的处理
			if newWidth < minWidth {
				scale := float64(minWidth) / float64(newWidth)
				newWidth = minWidth
				newHeight = int(float64(newHeight) * scale)
			}
			if newHeight < minHeight {
				scale := float64(minHeight) / float64(newHeight)
				newHeight = minHeight
				newWidth = int(float64(newWidth) * scale)
			}
		}
	}

	// 创建新的白色背景图片
	paddedImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	white := color.RGBA{255, 255, 255, 255}
	draw.Draw(paddedImg, paddedImg.Bounds(), &image.Uniform{white}, image.Point{}, draw.Src)

	// 计算原图在新图中的位置（居中）
	offsetX := (newWidth - originalWidth) / 2
	offsetY := (newHeight - originalHeight) / 2
	targetRect := image.Rect(offsetX, offsetY, offsetX+originalWidth, offsetY+originalHeight)

	// 将原图绘制到新图中央
	draw.Draw(paddedImg, targetRect, img, bounds.Min, draw.Src)

	// 编码图片
	var buf bytes.Buffer
	if err := pkgutils.EncodeImage(&buf, paddedImg, format, 95); err != nil {
		result.Error = fmt.Errorf("编码图片失败: %w", err)
		return result, result.Error
	}

	result.PaddedImage = buf.Bytes()
	result.NewWidth = newWidth
	result.NewHeight = newHeight
	result.Success = true

	return result, nil
}

// downloadImage 下载图片（使用统一的下载器）
func (p *ImagePaddingProcessor) downloadImage(imageURL string) (image.Image, string, error) {
	// 使用现有的图片下载器
	imageData, _, err := p.downloader.DownloadImage(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("下载图片失败: %w", err)
	}

	// 解码图片
	img, format, err := pkgutils.BytesToImageWithFormat(imageData)
	if err != nil {
		return nil, "", fmt.Errorf("解码图片失败: %w", err)
	}

	return img, format, nil
}
