package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// ImagePaddingProcessor 图片填充处理器
type ImagePaddingProcessor struct {
	logger *logrus.Entry
}

// NewImagePaddingProcessor 创建新的图片填充处理器
func NewImagePaddingProcessor() *ImagePaddingProcessor {
	return &ImagePaddingProcessor{
		logger: logrus.WithField("processor", "ImagePaddingProcessor"),
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

	// 确保满足最小尺寸要求
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
	if err := p.encodeImage(&buf, paddedImg, format); err != nil {
		result.Error = fmt.Errorf("编码图片失败: %w", err)
		return result, result.Error
	}

	result.PaddedImage = buf.Bytes()
	result.NewWidth = newWidth
	result.NewHeight = newHeight
	result.Success = true

	return result, nil
}

// downloadImage 下载图片
func (p *ImagePaddingProcessor) downloadImage(imageURL string) (image.Image, string, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	// 读取图片数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("解码图片失败: %w", err)
	}

	return img, format, nil
}

// encodeImage 编码图片
func (p *ImagePaddingProcessor) encodeImage(w io.Writer, img image.Image, format string) error {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 95})
	case "png":
		return png.Encode(w, img)
	default:
		// 默认使用JPEG
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 95})
	}
}

// CalculatePaddingDimensions 计算填充后的尺寸（不实际处理图片）
func (p *ImagePaddingProcessor) CalculatePaddingDimensions(width, height int, targetRatio float64, minWidth, minHeight int) (newWidth, newHeight int, needsPadding bool) {
	currentRatio := float64(width) / float64(height)

	// 检查是否需要填充 - 严格匹配，不允许容差
	if currentRatio == targetRatio && width >= minWidth && height >= minHeight {
		return width, height, false
	}

	// 计算新的尺寸
	if currentRatio > targetRatio {
		// 图片太宽，需要在上下添加白边
		newWidth = width
		newHeight = int(float64(width) / targetRatio)
	} else {
		// 图片太高，需要在左右添加白边
		newHeight = height
		newWidth = int(float64(height) * targetRatio)
	}

	// 确保满足最小尺寸要求
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

	return newWidth, newHeight, true
}
