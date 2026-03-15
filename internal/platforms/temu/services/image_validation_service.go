// Package services 提供TEMU平台图片验证统一服务
package services

import (
	"task-processor/internal/platforms/temu/api"
	temuimage "task-processor/internal/platforms/temu/api/image"

	"github.com/sirupsen/logrus"
)

// TemuProductProvider TEMU产品数据提供者接口（避免循环导入）
type TemuProductProvider interface {
	GetTemuProduct() *api.Product
	SetData(key string, value any)
}

// ImageValidationService 图片验证服务
type ImageValidationService struct {
	logger        *logrus.Entry
	configService *ImageConfigService
}

// NewImageValidationService 创建新的图片验证服务
func NewImageValidationService() *ImageValidationService {
	return &ImageValidationService{
		logger:        logrus.WithField("service", "ImageValidationService"),
		configService: NewImageConfigService(),
	}
}

// ValidateImage 验证单张图片
func (s *ImageValidationService) ValidateImage(imageURL string, requirement ImageRequirement) (*temuimage.ValidationResult, error) {
	result := &temuimage.ValidationResult{
		URL:         imageURL,
		IsValid:     false,
		Violations:  make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// TODO: 这里需要实际的图片信息获取逻辑
	// 暂时使用模拟数据，实际应该从图片URL获取真实的宽高、大小、格式信息

	return result, nil
}

// GetImageRequirement 获取图片要求配置
func (s *ImageValidationService) GetImageRequirement(provider TemuProductProvider) ImageRequirement {
	temuProduct := provider.GetTemuProduct()
	isClothes := temuProduct.GoodsBasic.IsClothes
	return s.configService.GetImageRequirement(isClothes)
}

// GetValidationSummary 获取图片验证摘要
func (s *ImageValidationService) GetValidationSummary(provider TemuProductProvider) map[string]any {
	temuProduct := provider.GetTemuProduct()
	summary := map[string]any{
		"main_images":     len(temuProduct.GoodsBasic.GoodsGallery.DetailImage),
		"sku_images":      0,
		"total_images":    0,
		"requires_upload": false,
	}

	skuImageCount := 0
	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			skuImageCount += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	summary["sku_images"] = skuImageCount
	summary["total_images"] = summary["main_images"].(int) + skuImageCount
	summary["requires_upload"] = summary["total_images"].(int) > 0

	return summary
}

// ValidateImageUploadRequirement 验证图片上传要求
func (s *ImageValidationService) ValidateImageUploadRequirement(provider TemuProductProvider) error {
	s.logger.Info("检查图片上传要求")

	temuProduct := provider.GetTemuProduct()
	// 检查是否需要调用 bg.local.goods.image.upload 进行转换
	totalImages := len(temuProduct.GoodsBasic.GoodsGallery.DetailImage)

	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			totalImages += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	if totalImages > 0 {
		// 设置标志，提醒后续处理器需要调用图片上传API
		provider.SetData("requires_image_upload", true)
		provider.SetData("total_image_count", totalImages)
		s.logger.Infof("检测到 %d 张图片需要处理", totalImages)
	}

	return nil
}
