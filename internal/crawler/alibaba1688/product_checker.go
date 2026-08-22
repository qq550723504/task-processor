// Package alibaba1688 提供1688产品检查功能
package alibaba1688

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"
)

type requiredFieldsError struct {
	err error
}

func (e *requiredFieldsError) Error() string {
	if e == nil || e.err == nil {
		return "必需字段缺失"
	}
	return e.err.Error()
}

func (e *requiredFieldsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newRequiredFieldsError(format string, args ...any) error {
	return &requiredFieldsError{err: fmt.Errorf(format, args...)}
}

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
		return newRequiredFieldsError("产品信息不能为空")
	}

	// 检查必需字段
	if err := pc.checkRequiredFields(product); err != nil {
		return fmt.Errorf("必需字段检查失败: %w", err)
	}
	if err := pc.validateSupplier(product); err != nil {
		return fmt.Errorf("供应商信息验证失败: %w", err)
	}
	if err := pc.validateProductMetrics(product); err != nil {
		return fmt.Errorf("商品数值信息验证失败: %w", err)
	}
	if err := pc.validateVariants(product); err != nil {
		return fmt.Errorf("商品变体信息验证失败: %w", err)
	}
	if err := pc.validatePackInfo(product); err != nil {
		return fmt.Errorf("商品包装信息验证失败: %w", err)
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

func (pc *ProductChecker) validateSupplier(product *model.Product1688) error {
	supplier := product.Supplier
	if strings.TrimSpace(supplier.ShopURL) != "" && !isValidSupplierShopURL(supplier.ShopURL) {
		return fmt.Errorf("供应商店铺URL格式无效")
	}
	if supplier.YearsInBusiness < 0 {
		return fmt.Errorf("经营年限不能为负数")
	}
	if !isFiniteInRange(supplier.Rating, 0, 5) {
		return fmt.Errorf("供应商评分必须在0到5之间")
	}
	if !isFiniteInRange(supplier.ResponseRate, 0, 100) {
		return fmt.Errorf("供应商响应率必须在0到100之间")
	}
	return nil
}

func isValidSupplierShopURL(raw string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func (pc *ProductChecker) validateProductMetrics(product *model.Product1688) error {
	if product.SalesVolume < 0 {
		return fmt.Errorf("销量不能为负数")
	}
	if product.ReviewCount < 0 {
		return fmt.Errorf("评价数不能为负数")
	}
	if !isFiniteInRange(product.Rating, 0, 5) {
		return fmt.Errorf("商品评分必须在0到5之间")
	}
	return nil
}

func (pc *ProductChecker) validateVariants(product *model.Product1688) error {
	for i, variant := range product.Variants {
		for key := range variant.Attributes {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "stock", "price":
				return fmt.Errorf("变体[%d]属性不能覆盖保留数值字段: %s", i, key)
			}
		}
		if variant.Stock < 0 {
			return fmt.Errorf("变体[%d]库存不能为负数", i)
		}
		if math.IsNaN(variant.Price) || math.IsInf(variant.Price, 0) || variant.Price < 0 {
			return fmt.Errorf("变体[%d]价格不能为负数或非有限值", i)
		}
	}
	return nil
}

func (pc *ProductChecker) validatePackInfo(product *model.Product1688) error {
	if product.PackInfo == nil {
		return nil
	}
	if math.IsNaN(product.PackInfo.Weight) || math.IsInf(product.PackInfo.Weight, 0) || product.PackInfo.Weight < 0 {
		return fmt.Errorf("包装重量不能为负数或非有限值")
	}
	return nil
}

func isFiniteInRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}

// checkRequiredFields 检查必需字段
func (pc *ProductChecker) checkRequiredFields(product *model.Product1688) error {
	if strings.TrimSpace(product.Title) == "" {
		return newRequiredFieldsError("标题不能为空")
	}
	if math.IsNaN(product.MinPrice) || math.IsInf(product.MinPrice, 0) {
		return fmt.Errorf("最低价格必须是有限数值")
	}
	if product.MinPrice == 0 {
		return newRequiredFieldsError("最低价格必须大于0")
	}
	if product.MinPrice < 0 {
		return fmt.Errorf("最低价格不能为负数")
	}
	if product.MinOrderQuantity == 0 {
		return newRequiredFieldsError("起订量必须大于0")
	}
	if product.MinOrderQuantity < 0 {
		return fmt.Errorf("起订量不能为负数")
	}
	if strings.TrimSpace(product.Supplier.Name) == "" {
		return newRequiredFieldsError("供应商名称不能为空")
	}
	if strings.TrimSpace(product.URL) == "" {
		return newRequiredFieldsError("商品URL不能为空")
	}

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
	if math.IsNaN(product.MaxPrice) || math.IsInf(product.MaxPrice, 0) || product.MaxPrice < 0 {
		return fmt.Errorf("最高价格不能为负数或非有限值")
	}
	// 检查价格范围
	if product.MinPrice > product.MaxPrice && product.MaxPrice > 0 {
		return fmt.Errorf("最低价格不能大于最高价格")
	}

	// 检查价格阶梯
	if len(product.PriceRanges) > 0 {
		for i, priceRange := range product.PriceRanges {
			if math.IsNaN(priceRange.Price) || math.IsInf(priceRange.Price, 0) || priceRange.Price <= 0 {
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
	for i, video := range product.Videos {
		if strings.TrimSpace(video.VideoURL) != "" && !isValidMediaURL(video.VideoURL) {
			return fmt.Errorf("视频[%d]URL格式无效: %s", i, video.VideoURL)
		}
		if strings.TrimSpace(video.CoverURL) != "" && !pc.isValidImageURL(video.CoverURL) {
			return fmt.Errorf("视频[%d]封面URL格式无效: %s", i, video.CoverURL)
		}
	}
	for i, detail := range product.ProductDetails {
		for j, imageURL := range detail.Images {
			if !pc.isValidImageURL(imageURL) {
				return fmt.Errorf("详情[%d]图片[%d]URL格式无效: %s", i, j, imageURL)
			}
		}
	}
	for i, variant := range product.Variants {
		if strings.TrimSpace(variant.Image) != "" && !pc.isValidImageURL(variant.Image) {
			return fmt.Errorf("变体[%d]图片URL格式无效: %s", i, variant.Image)
		}
	}
	if product.PackInfo != nil {
		for i, imageURL := range product.PackInfo.PackageImages {
			if !pc.isValidImageURL(imageURL) {
				return fmt.Errorf("包装图片[%d]URL格式无效: %s", i, imageURL)
			}
		}
	}

	return nil
}

func isValidMediaURL(raw string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

// isValidImageURL 检查图片URL是否有效
func (pc *ProductChecker) isValidImageURL(imageURL string) bool {
	if !isValidMediaURL(imageURL) {
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
