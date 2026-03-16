// Package image 提供TEMU平台图片尺寸标注功能
package image

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"task-processor/internal/platforms/temu/handlers/rules"

	"github.com/sirupsen/logrus"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/downloader"
	"task-processor/internal/pkg/imagex"
)

// ImageDimensionAnnotator 图片尺寸标注器
type ImageDimensionAnnotator struct {
	logger         *logrus.Entry
	openaiClient   *openaiClient.Client
	downloader     *downloader.ImageDownloader
	drawer         *DimensionDrawer
	textRenderer   *rules.TextRenderer
	visionDetector *VisionDetector
}

// NewImageDimensionAnnotator 创建新的图片尺寸标注器
func NewImageDimensionAnnotator() *ImageDimensionAnnotator {
	return &ImageDimensionAnnotator{
		logger:         logrus.WithField("component", "ImageDimensionAnnotator"),
		downloader:     downloader.NewImageDownloader(),
		drawer:         NewDimensionDrawer(),
		textRenderer:   rules.NewTextRenderer(),
		visionDetector: NewVisionDetector(nil),
	}
}

// NewImageDimensionAnnotatorWithOpenAI 创建带OpenAI支持的图片尺寸标注器
func NewImageDimensionAnnotatorWithOpenAI(client *openaiClient.Client) *ImageDimensionAnnotator {
	return &ImageDimensionAnnotator{
		logger:         logrus.WithField("component", "ImageDimensionAnnotator"),
		openaiClient:   client,
		downloader:     downloader.NewImageDownloader(),
		drawer:         NewDimensionDrawer(),
		textRenderer:   rules.NewTextRenderer(),
		visionDetector: NewVisionDetector(client),
	}
}

// AnnotateImage 为图片添加尺寸标注（从URL下载）
func (a *ImageDimensionAnnotator) AnnotateImage(imageURL string, dimensions DimensionInfo) ([]byte, error) {
	a.logger.Infof("开始为图片添加尺寸标注: %s", imageURL)

	// 1. 下载并解码图片
	img, format, err := a.DownloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	return a.annotateImageInternal(img, format, dimensions)
}

// AnnotateImageFromBytes 为图片添加尺寸标注（从字节数据）
func (a *ImageDimensionAnnotator) AnnotateImageFromBytes(imageData []byte, dimensions DimensionInfo) ([]byte, error) {
	a.logger.Info("开始为图片添加尺寸标注（使用字节数据）")

	// 1. 解码图片
	img, format, err := imagex.FromBytesWithFormat(imageData)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	return a.annotateImageInternal(img, format, dimensions)
}

// DownloadImage 下载图片（公开方法）
func (a *ImageDimensionAnnotator) DownloadImage(imageURL string) (image.Image, string, error) {
	// 使用现有的图片下载器
	imageData, _, err := a.downloader.DownloadImage(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("下载图片失败: %w", err)
	}

	// 解码图片
	img, format, err := imagex.FromBytesWithFormat(imageData)
	if err != nil {
		return nil, "", fmt.Errorf("解码图片失败: %w", err)
	}

	return img, format, nil
}

// HasDimensionAnnotationWithDetails 检测图片是否已包含尺寸标注（带详细信息，公开方法）
func (a *ImageDimensionAnnotator) HasDimensionAnnotationWithDetails(ctx context.Context, img image.Image) (bool, string) {
	return a.visionDetector.HasDimensionAnnotationWithDetails(ctx, img)
}

// annotateImageInternal 内部标注方法
func (a *ImageDimensionAnnotator) annotateImageInternal(img image.Image, format string, dimensions DimensionInfo) ([]byte, error) {
	// 1. 暂时跳过检测，直接添加标注
	a.logger.Info("⚠️ 跳过检测，直接添加尺寸标注")

	// 2. 创建可绘制的图片
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// 3. 绘制尺寸标注
	if err := a.drawer.DrawDimensionAnnotations(rgba, dimensions); err != nil {
		return nil, fmt.Errorf("绘制标注失败: %w", err)
	}

	// 4. 编码为字节
	buf := new(bytes.Buffer)
	if err := imagex.Encode(buf, rgba, format, 95); err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	a.logger.Infof("✅ 尺寸标注完成，图片大小: %d bytes", buf.Len())
	return buf.Bytes(), nil
}
