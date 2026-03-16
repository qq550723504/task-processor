// Package image 提供TEMU平台图片初始化处理器
package image

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/ptrutil"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageInitHandler 图片初始化处理器 - 从Amazon产品中提取图片URL
type ImageInitHandler struct {
	logger *logrus.Entry
}

// NewImageInitHandler 创建新的图片初始化处理器
func NewImageInitHandler() *ImageInitHandler {
	return &ImageInitHandler{
		logger: logger.GetGlobalLogger("temu.handlers.image_init"),
	}
}

// Name 返回处理器名称
func (h *ImageInitHandler) Name() string {
	return "图片初始化处理器"
}

// HandleTemu 处理任务（强类型上下文）
func (h *ImageInitHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始初始化产品图片URL")

	// 获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 获取Amazon产品信息
	var amazonProduct *model.Product
	if amazonCtx, ok := any(temuCtx.DefaultTaskContext).(pipeline.AmazonContext); ok {
		amazonProduct = amazonCtx.GetAmazonProduct()
	}

	if amazonProduct == nil || len(amazonProduct.Images) == 0 {
		h.logger.Warn("Amazon产品没有图片")
		return nil
	}

	// 初始化主图（DetailImage）
	var detailImages []models.ImageInfo
	for _, imgURL := range amazonProduct.Images {
		if imgURL != "" {
			detailImages = append(detailImages, models.ImageInfo{
				URL:    imgURL,
				Width:  0, // 尺寸将在验证时获取
				Height: 0,
				Type:   ptrutil.IntPtr(1),
			})
		}
	}

	// 设置到产品数据中
	temuProduct.GoodsBasic.GoodsGallery.DetailImage = detailImages

	h.logger.WithFields(logrus.Fields{
		"image_count": len(detailImages),
	}).Info("初始化主图URL完成")

	// 初始化SKU图片（如果已经有SKU的话）
	h.initSkuImages(temuCtx, temuProduct)

	return nil
}

// initSkuImages 初始化SKU图片
func (h *ImageInitHandler) initSkuImages(temuCtx *temucontext.TemuTaskContext, temuProduct *models.Product) {
	if len(temuProduct.SkcList) == 0 {
		h.logger.Debug("暂无SKU，跳过SKU图片初始化")
		return
	}

	// 获取Amazon产品信息
	var amazonProduct *model.Product
	if amazonCtx, ok := any(temuCtx.DefaultTaskContext).(pipeline.AmazonContext); ok {
		amazonProduct = amazonCtx.GetAmazonProduct()
	}

	totalInitialized := 0

	for skcIndex := range temuProduct.SkcList {
		for skuIndex := range temuProduct.SkcList[skcIndex].SkuList {
			sku := &temuProduct.SkcList[skcIndex].SkuList[skuIndex]

			// 如果SKU已经有图片，跳过
			if len(sku.CarouselGallery) > 0 {
				continue
			}

			// 从Amazon产品中获取图片
			var carouselImages []models.ImageInfo
			if amazonProduct != nil {
				for _, imgURL := range amazonProduct.Images {
					if imgURL != "" {
						carouselImages = append(carouselImages, models.ImageInfo{
							URL:    imgURL,
							Width:  0,
							Height: 0,
							Type:   ptrutil.IntPtr(1),
						})
					}
				}
			}

			sku.CarouselGallery = carouselImages
			totalInitialized += len(carouselImages)
		}
	}

	if totalInitialized > 0 {
		h.logger.WithFields(logrus.Fields{
			"image_count": totalInitialized,
		}).Info("初始化SKU轮播图URL完成")
	}
}
