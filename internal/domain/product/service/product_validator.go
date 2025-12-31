// Package service 提供产品数据验证功能
package service

import (
	"fmt"
	"strings"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product/types"

	"github.com/sirupsen/logrus"
)

// ProductValidator 产品数据验证器
type ProductValidator struct {
	logger *logrus.Entry
}

// NewProductValidator 创建产品数据验证器
func NewProductValidator(logger *logrus.Entry) *ProductValidator {
	return &ProductValidator{
		logger: logger.WithField("component", "ProductValidator"),
	}
}

// ValidateRequest 验证请求参数
func (v *ProductValidator) ValidateRequest(req *types.FetchRequest) error {
	if req == nil {
		return fmt.Errorf("请求参数不能为空")
	}

	// 基本字段验证
	if req.TenantID <= 0 {
		return fmt.Errorf("TenantID必须大于0")
	}

	if strings.TrimSpace(req.Platform) == "" {
		return fmt.Errorf("Platform不能为空")
	}

	if strings.TrimSpace(req.Region) == "" {
		return fmt.Errorf("Region不能为空")
	}

	if strings.TrimSpace(req.ProductID) == "" {
		return fmt.Errorf("ProductID不能为空")
	}

	// 平台验证
	if !v.validatePlatform(req.Platform) {
		return fmt.Errorf("不支持的平台: %s", req.Platform)
	}

	// 地区验证
	if !v.validateRegion(req.Region) {
		return fmt.Errorf("不支持的地区: %s", req.Region)
	}

	// 额外的业务验证
	if err := v.validateBusinessRules(req); err != nil {
		return err
	}

	return nil
}

// ValidateProduct 验证产品数据
func (v *ProductValidator) ValidateProduct(product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据不能为空")
	}

	// 基本字段验证
	if strings.TrimSpace(product.Asin) == "" {
		return fmt.Errorf("产品ASIN不能为空")
	}

	if strings.TrimSpace(product.Title) == "" {
		return fmt.Errorf("产品标题不能为空")
	}

	// 价格验证
	if err := v.validatePrice(product); err != nil {
		return err
	}

	// 图片验证
	if err := v.validateImages(product); err != nil {
		return err
	}

	return nil
}

// validateBusinessRules 验证业务规则
func (v *ProductValidator) validateBusinessRules(req *types.FetchRequest) error {
	// 验证ProductID格式
	if err := v.validateProductIDFormat(req.Platform, req.ProductID); err != nil {
		return err
	}

	// 验证平台和地区的组合
	if err := v.validatePlatformRegionCombination(req.Platform, req.Region); err != nil {
		return err
	}

	return nil
}

// validateProductIDFormat 验证产品ID格式
func (v *ProductValidator) validateProductIDFormat(platform, productID string) error {
	platform = strings.ToLower(platform)

	switch platform {
	case "amazon":
		// Amazon ASIN格式验证：10位字母数字组合
		if len(productID) != 10 {
			return fmt.Errorf("amazon ASIN必须是10位字符")
		}
		for _, char := range productID {
			if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
				return fmt.Errorf("amazon ASIN只能包含大写字母和数字")
			}
		}
	case "temu":
		// TEMU产品ID验证：数字
		if len(productID) == 0 {
			return fmt.Errorf("TEMU产品ID不能为空")
		}
	case "shein":
		// SHEIN产品ID验证
		if len(productID) == 0 {
			return fmt.Errorf("SHEIN产品ID不能为空")
		}
	}

	return nil
}

// validatePlatformRegionCombination 验证平台和地区组合
func (v *ProductValidator) validatePlatformRegionCombination(platform, region string) error {
	supportedCombinations := map[string][]string{
		"amazon": {"us", "uk", "de", "fr", "it", "es", "ca", "jp", "au", "mx", "ae", "sa"},
		"temu":   {"us", "uk", "de", "fr", "it", "es", "ca", "jp", "au"},
		"shein":  {"us", "uk", "de", "fr", "it", "es"},
	}

	platform = strings.ToLower(platform)
	region = strings.ToLower(region)

	if regions, exists := supportedCombinations[platform]; exists {
		for _, supportedRegion := range regions {
			if region == supportedRegion {
				return nil
			}
		}
		return fmt.Errorf("平台 %s 不支持地区 %s", platform, region)
	}

	return fmt.Errorf("不支持的平台: %s", platform)
}

// validatePrice 验证价格信息
func (v *ProductValidator) validatePrice(product *model.Product) error {
	// 检查价格是否为负数
	if product.FinalPrice < 0 {
		return fmt.Errorf("产品价格不能为负数: %f", product.FinalPrice)
	}

	// 检查原价是否合理（需要先检查指针是否为nil）
	if product.PricesBreakdown.ListPrice != nil && *product.PricesBreakdown.ListPrice < 0 {
		return fmt.Errorf("产品原价不能为负数: %f", *product.PricesBreakdown.ListPrice)
	}

	// 检查价格逻辑（需要先检查指针是否为nil）
	if product.PricesBreakdown.ListPrice != nil && *product.PricesBreakdown.ListPrice > 0 &&
		product.InitialPrice > product.FinalPrice {
		v.logger.Warnf("产品 %s 当前价格(%f)高于原价(%f)",
			product.Asin, product.FinalPrice, *product.PricesBreakdown.ListPrice)
	}

	return nil
}

// validateImages 验证图片信息
func (v *ProductValidator) validateImages(product *model.Product) error {
	// 检查主图
	if strings.TrimSpace(product.ImageURL) == "" {
		return fmt.Errorf("产品主图不能为空")
	}

	// 验证图片URL格式
	if !strings.HasPrefix(product.ImageURL, "http") {
		return fmt.Errorf("产品主图URL格式不正确: %s", product.ImageURL)
	}

	// 检查附加图片
	for i, img := range product.Images {
		if strings.TrimSpace(img) == "" {
			v.logger.Warnf("产品 %s 第%d张图片为空", product.Asin, i+1)
			continue
		}
		if !strings.HasPrefix(img, "http") {
			v.logger.Warnf("产品 %s 第%d张图片URL格式不正确: %s", product.Asin, i+1, img)
		}
	}

	return nil
}

// validatePlatform 验证平台
func (v *ProductValidator) validatePlatform(platform string) bool {
	platform = strings.ToLower(platform)
	supportedPlatforms := []string{"amazon", "temu", "shein"}

	for _, supported := range supportedPlatforms {
		if platform == supported {
			return true
		}
	}
	return false
}

// validateRegion 验证地区
func (v *ProductValidator) validateRegion(region string) bool {
	region = strings.ToLower(region)
	supportedRegions := []string{
		"us", "uk", "de", "fr", "it", "es", "ca", "jp", "au", "mx", "ae", "sa",
	}

	for _, supported := range supportedRegions {
		if region == supported {
			return true
		}
	}
	return false
}
