package handlers

import (
	"context"
	"fmt"
	"image"

	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/watermark"

	"github.com/sirupsen/logrus"
)

// WatermarkHandler 水印处理器
type WatermarkHandler struct {
	*pipeline.BaseHandler
	processor *watermark.Processor
}

// NewWatermarkHandler 创建水印处理器
func NewWatermarkHandler(config *watermark.Config, logger *logrus.Logger) pipeline.Handler {
	if logger == nil {
		logger = logrus.New()
	}

	return &WatermarkHandler{
		BaseHandler: pipeline.NewBaseHandler("水印处理器"),
		processor:   watermark.NewProcessor(config, logger),
	}
}

// Handle 处理水印
func (h *WatermarkHandler) Handle(ctx pipeline.TaskContext) error {
	h.LogStart()
	h.GetLogger().Info("开始水印处理...")

	// 从上下文获取图片列表
	images, ok := ctx.GetData("images")
	if !ok {
		h.GetLogger().Debug("上下文中没有图片，跳过水印处理")
		h.LogSuccess()
		return nil
	}

	// 处理图片列表
	switch imgs := images.(type) {
	case []image.Image:
		ctx.SetData("images", h.processImages(ctx.GetContext(), imgs))

	case image.Image:
		processedImage, err := h.processSingleImage(ctx.GetContext(), imgs)
		if err != nil {
			h.LogError(err)
			return fmt.Errorf("处理图片失败: %w", err)
		}
		ctx.SetData("images", processedImage)

	case []string:
		// 如果是图片URL列表，需要先下载
		h.GetLogger().Debug("检测到图片URL列表，需要先下载图片")
		// 这里可以集成下载逻辑，暂时跳过

	default:
		h.GetLogger().Warnf("不支持的图片类型: %T", images)
	}

	h.LogSuccess()
	return nil
}

// processImages 批量处理图片
func (h *WatermarkHandler) processImages(ctx context.Context, images []image.Image) []image.Image {
	processedImages := make([]image.Image, 0, len(images))
	var stats struct {
		total    int
		detected int
		removed  int
		failed   int
	}

	stats.total = len(images)

	for i, img := range images {
		h.GetLogger().Debugf("处理第 %d/%d 张图片", i+1, len(images))

		result, err := h.processor.Process(ctx, img)
		if err != nil {
			h.GetLogger().Errorf("处理图片失败: %v", err)
			stats.failed++
			// 失败时使用原图
			processedImages = append(processedImages, img)
			continue
		}

		// 统计
		if result.Detection.HasWatermark {
			stats.detected++
		}
		if result.Removal != nil && result.Removal.Success {
			stats.removed++
			processedImages = append(processedImages, result.Removal.Image)
		} else {
			processedImages = append(processedImages, img)
		}
	}

	h.GetLogger().Infof("水印处理完成: 总数=%d, 检测到=%d, 已去除=%d, 失败=%d",
		stats.total, stats.detected, stats.removed, stats.failed)

	return processedImages
}

// processSingleImage 处理单张图片
func (h *WatermarkHandler) processSingleImage(ctx context.Context, img image.Image) (image.Image, error) {
	result, err := h.processor.Process(ctx, img)
	if err != nil {
		return img, err
	}

	if result.Removal != nil && result.Removal.Success {
		h.GetLogger().Infof("水印去除成功: 质量=%.2f", result.Removal.Quality)
		return result.Removal.Image, nil
	}

	return img, nil
}
