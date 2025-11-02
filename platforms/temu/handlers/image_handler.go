package handlers

import (
	"fmt"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ImageHandler 图片处理器
type ImageHandler struct {
	logger *logrus.Entry
}

// NewImageHandler 创建新的图片处理器
func NewImageHandler() *ImageHandler {
	return &ImageHandler{
		logger: logrus.WithField("handler", "ImageHandler"),
	}
}

// Name 返回处理器名称
func (h *ImageHandler) Name() string {
	return "图片处理器"
}

// Handle 处理任务
func (h *ImageHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理产品图片")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 处理产品图片
	err := h.processProductImages(ctx)
	if err != nil {
		h.logger.Errorf("处理产品图片失败: %v", err)
		return fmt.Errorf("处理产品图片失败: %w", err)
	}

	h.logger.Info("产品图片处理完成")
	return nil
}

// processProductImages 处理产品图片
func (h *ImageHandler) processProductImages(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理产品图片信息")

	// 验证主图
	err := h.validateMainImage(ctx)
	if err != nil {
		h.logger.Errorf("主图验证失败: %v", err)
		return err
	}

	// 处理详情图片
	err = h.processDetailImages(ctx)
	if err != nil {
		h.logger.Errorf("处理详情图片失败: %v", err)
		return err
	}

	// 处理SKU图片
	err = h.processSkuImages(ctx)
	if err != nil {
		h.logger.Errorf("处理SKU图片失败: %v", err)
		return err
	}

	h.logger.Info("产品图片处理完成")
	return nil
}

// validateMainImage 验证主图
func (h *ImageHandler) validateMainImage(ctx *pipeline.TaskContext) error {
	mainImageURL := ctx.TemuProduct.GoodsBasic.HdThumbURL
	if mainImageURL == "" {
		return fmt.Errorf("主图URL为空")
	}

	h.logger.Infof("验证主图: %s", mainImageURL)

	// 这里可以添加图片验证逻辑，比如：
	// - 检查图片格式
	// - 检查图片尺寸
	// - 检查图片质量
	// - 检查图片内容合规性

	h.logger.Info("主图验证通过")
	return nil
}

// processDetailImages 处理详情图片
func (h *ImageHandler) processDetailImages(ctx *pipeline.TaskContext) error {
	detailImages := ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage
	if len(detailImages) == 0 {
		h.logger.Warn("未找到详情图片")
		return nil
	}

	h.logger.Infof("处理 %d 张详情图片", len(detailImages))

	// 验证每张详情图片
	validImages := make([]types.ImageInfo, 0)
	for i, image := range detailImages {
		if h.validateImage(image, fmt.Sprintf("详情图片[%d]", i+1)) {
			validImages = append(validImages, image)
		}
	}

	// 更新有效的详情图片
	ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage = validImages

	h.logger.Infof("详情图片处理完成: %d 张有效图片", len(validImages))
	return nil
}

// processSkuImages 处理SKU图片
func (h *ImageHandler) processSkuImages(ctx *pipeline.TaskContext) error {
	skcList := ctx.TemuProduct.SkcList
	if len(skcList) == 0 {
		h.logger.Info("未找到SKC列表，跳过SKU图片处理")
		return nil
	}

	h.logger.Infof("处理 %d 个SKC的图片", len(skcList))

	for i, skc := range skcList {
		// 处理SKC的轮播图片
		if len(skc.CarouselGallery) > 0 {
			validCarouselImages := make([]types.ImageInfo, 0)
			for j, image := range skc.CarouselGallery {
				if h.validateImage(image, fmt.Sprintf("SKC[%d]轮播图片[%d]", i+1, j+1)) {
					validCarouselImages = append(validCarouselImages, image)
				}
			}
			ctx.TemuProduct.SkcList[i].CarouselGallery = validCarouselImages
		}

		// 处理SKC的颜色图片
		if skc.ColorImageUrl != "" {
			h.logger.Infof("处理SKC[%d]颜色图片: %s", i+1, skc.ColorImageUrl)
		}

		// 处理SKU图片
		for j, sku := range skc.SkuList {
			if len(sku.CarouselGallery) > 0 {
				validSkuImages := make([]types.ImageInfo, 0)
				for k, image := range sku.CarouselGallery {
					if h.validateImage(image, fmt.Sprintf("SKU[%d-%d]图片[%d]", i+1, j+1, k+1)) {
						validSkuImages = append(validSkuImages, image)
					}
				}
				ctx.TemuProduct.SkcList[i].SkuList[j].CarouselGallery = validSkuImages
			}
		}
	}

	h.logger.Info("SKU图片处理完成")
	return nil
}

// validateImage 验证单张图片
func (h *ImageHandler) validateImage(image types.ImageInfo, imageName string) bool {
	if image.URL == "" {
		h.logger.Warnf("%s URL为空", imageName)
		return false
	}

	if image.Width <= 0 || image.Height <= 0 {
		h.logger.Warnf("%s 尺寸无效: %dx%d", imageName, image.Width, image.Height)
		return false
	}

	// 检查图片尺寸要求
	if image.Width < 300 || image.Height < 300 {
		h.logger.Warnf("%s 尺寸过小: %dx%d (最小要求: 300x300)", imageName, image.Width, image.Height)
		return false
	}

	h.logger.Debugf("%s 验证通过: %s (%dx%d)", imageName, image.URL, image.Width, image.Height)
	return true
}
