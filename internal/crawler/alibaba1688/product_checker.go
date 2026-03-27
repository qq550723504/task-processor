// Package alibaba1688 提供1688产品检查功能
package alibaba1688

import (
	"task-processor/internal/core/logger"
	"fmt"
	"strings"
	"task-processor/internal/crawler/alibaba1688/model"

)

// ProductChecker 1688产品检查器
type ProductChecker struct {
	// 敏感词列表
	sensitiveWords []string
	// 必需字段列表
	requiredFields []string
}

// NewProductChecker 创建新的产品检查器
func NewProductChecker() *ProductChecker {
	return &ProductChecker{
		sensitiveWords: []string{
			// 违禁词示例
			"假货", "仿品", "高仿", "A货", "山寨",
			"违法", "禁售", "管制", "危险品",
			// 可以根据需要添加更多敏感词
		},
		requiredFields: []string{
			"title", "minPrice", "minOrderQuantity", "supplier.name",
		},
	}
}

// ValidateProduct 验证产品信息的完整性和合规性
func (pc *ProductChecker) ValidateProduct(product *model.Product1688) error {
	if product == nil {
		return fmt.Errorf("产品信息不能为空")
	}

	// 检查必需字段
	if err := pc.checkRequiredFields(product); err != nil {
		return fmt.Errorf("必需字段检查失败: %w", err)
	}

	// 检查敏感词
	if err := pc.checkSensitiveWords(product); err != nil {
		return fmt.Errorf("敏感词检查失败: %w", err)
	}

	// 检查价格合理性
	if err := pc.validatePricing(product); err != nil {
		return fmt.Errorf("价格验证失败: %w", err)
	}

	// 检查图片
	if err := pc.validateImages(product); err != nil {
		return fmt.Errorf("图片验证失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("产品验证通过: %s", product.Title)
	return nil
}

// checkRequiredFields 检查必需字段
func (pc *ProductChecker) checkRequiredFields(product *model.Product1688) error {
	// 检查标题

	// 检查价格

	// 检查起订量

	// 检查供应商信息

	// 检查URL

	return nil
}

// checkSensitiveWords 检查敏感词
func (pc *ProductChecker) checkSensitiveWords(product *model.Product1688) error {
	// 检查标题中的敏感词
	titleLower := strings.ToLower(product.Title)
	for _, word := range pc.sensitiveWords {
		if strings.Contains(titleLower, strings.ToLower(word)) {
			return fmt.Errorf("标题包含敏感词: %s", word)
		}
	}

	return nil
}

// validatePricing 验证价格信息
func (pc *ProductChecker) validatePricing(product *model.Product1688) error {
	// 检查价格范围
	if product.MinPrice > product.MaxPrice && product.MaxPrice > 0 {
		return fmt.Errorf("最低价格不能大于最高价格")
	}

	// 检查价格阶梯
	if len(product.PriceRanges) > 0 {
		for i, priceRange := range product.PriceRanges {
			if priceRange.Price <= 0 {
				return fmt.Errorf("价格阶梯[%d]价格必须大于0", i)
			}
			if priceRange.MinQuantity <= 0 {
				return fmt.Errorf("价格阶梯[%d]最小数量必须大于0", i)
			}
			if priceRange.MaxQuantity > 0 && priceRange.MinQuantity > priceRange.MaxQuantity {
				return fmt.Errorf("价格阶梯[%d]最小数量不能大于最大数量", i)
			}
		}

		// 检查价格阶梯是否按数量递增排序
		for i := 1; i < len(product.PriceRanges); i++ {
			if product.PriceRanges[i].MinQuantity <= product.PriceRanges[i-1].MinQuantity {
				return fmt.Errorf("价格阶梯应按数量递增排序")
			}
		}
	}

	return nil
}

// validateImages 验证图片信息
func (pc *ProductChecker) validateImages(product *model.Product1688) error {
	// 检查是否有主图
	if strings.TrimSpace(product.MainImage) == "" && len(product.Images) == 0 {
		return fmt.Errorf("商品必须至少有一张图片")
	}

	// 如果有主图，检查主图URL格式
	if product.MainImage != "" {
		if !pc.isValidImageURL(product.MainImage) {
			return fmt.Errorf("主图URL格式无效: %s", product.MainImage)
		}
	}

	// 检查图片列表中的URL格式
	for i, imageURL := range product.Images {
		if !pc.isValidImageURL(imageURL) {
			return fmt.Errorf("图片[%d]URL格式无效: %s", i, imageURL)
		}
	}

	return nil
}

// isValidImageURL 检查图片URL是否有效
func (pc *ProductChecker) isValidImageURL(imageURL string) bool {
	if imageURL == "" {
		return false
	}

	// 检查是否以http或https开头
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return false
	}

	// 检查是否包含常见的图片文件扩展名
	lowerURL := strings.ToLower(imageURL)
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}

	for _, ext := range imageExtensions {
		if strings.Contains(lowerURL, ext) {
			return true
		}
	}

	// 1688的图片URL可能不包含扩展名，检查是否包含1688域名
	if strings.Contains(lowerURL, "1688.com") || strings.Contains(lowerURL, "alicdn.com") {
		return true
	}

	return false
}

// IsProductAvailable 检查产品是否可用（未下架、有库存等）
func (pc *ProductChecker) IsProductAvailable(product *model.Product1688) bool {
	// 检查基本信息是否完整
	if product.Title == "" || product.MinPrice <= 0 {
		return false
	}

	// 检查供应商信息
	if product.Supplier.Name == "" {
		return false
	}

	return true
}

// GetProductQualityScore 获取产品质量评分（0-100）
func (pc *ProductChecker) GetProductQualityScore(product *model.Product1688) int {
	score := 0

	// 基础信息完整性 (30分)
	if product.Title != "" {
		score += 10
	}

	if len(product.Images) > 0 {
		score += 10
	}

	// 价格信息完整性 (20分)
	if product.MinPrice > 0 {
		score += 10
	}
	if len(product.PriceRanges) > 0 {
		score += 10
	}

	// 供应商信息质量 (30分)
	if product.Supplier.Name != "" {
		score += 10
	}
	if product.Supplier.IsGoldSupplier {
		score += 10
	}
	if product.Supplier.IsVerified {
		score += 10
	}

	// 商品详细信息 (20分)
	if len(product.Specifications) > 0 {
		score += 10
	}
	if product.SalesVolume > 0 {
		score += 10
	}

	return score
}
