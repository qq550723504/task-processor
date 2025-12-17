package handlers

import (
	"fmt"

	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ImageInitHandler 图片初始化处理器 - 从Amazon产品中提取图片URL
type ImageInitHandler struct {
	logger *logrus.Entry
}

// NewImageInitHandler 创建新的图片初始化处理器
func NewImageInitHandler() *ImageInitHandler {
	return &ImageInitHandler{
		logger: logrus.WithField("handler", "ImageInitHandler"),
	}
}

// Name 返回处理器名称
func (h *ImageInitHandler) Name() string {
	return "图片初始化处理器"
}

// Handle 处理任务
func (h *ImageInitHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始初始化产品图片URL")

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	if ctx.AmazonProduct == nil || len(ctx.AmazonProduct.Images) == 0 {
		h.logger.Warn("Amazon产品没有图片")
		return nil
	}

	// 初始化主图（DetailImage）
	var detailImages []types.ImageInfo
	for _, imgURL := range ctx.AmazonProduct.Images {
		if imgURL != "" {
			detailImages = append(detailImages, types.ImageInfo{
				URL:    imgURL,
				Width:  0, // 尺寸将在验证时获取
				Height: 0,
				Type:   intPtr(1),
			})
		}
	}

	// 设置到产品数据中
	ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage = detailImages

	h.logger.Infof("初始化了 %d 张主图URL", len(detailImages))

	// 初始化SKU图片（如果已经有SKU的话）
	h.initSkuImages(ctx)

	return nil
}

// initSkuImages 初始化SKU图片
func (h *ImageInitHandler) initSkuImages(ctx *pipeline.TaskContext) {
	if len(ctx.TemuProduct.SkcList) == 0 {
		h.logger.Debug("暂无SKU，跳过SKU图片初始化")
		return
	}

	totalInitialized := 0

	for skcIndex := range ctx.TemuProduct.SkcList {
		for skuIndex := range ctx.TemuProduct.SkcList[skcIndex].SkuList {
			sku := &ctx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex]

			// 如果SKU已经有图片，跳过
			if len(sku.CarouselGallery) > 0 {
				continue
			}

			// 从Amazon产品中获取图片
			var carouselImages []types.ImageInfo
			if ctx.AmazonProduct != nil {
				for _, imgURL := range ctx.AmazonProduct.Images {
					if imgURL != "" {
						carouselImages = append(carouselImages, types.ImageInfo{
							URL:    imgURL,
							Width:  0,
							Height: 0,
							Type:   intPtr(1),
						})
					}
				}
			}

			sku.CarouselGallery = carouselImages
			totalInitialized += len(carouselImages)
		}
	}

	if totalInitialized > 0 {
		h.logger.Infof("初始化了 %d 张SKU轮播图URL", totalInitialized)
	}
}
