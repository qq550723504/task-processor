// Package product 提供产品数据验证功能
package product

import (
	"fmt"
	"strings"
	"task-processor/internal/model"

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
func (v *ProductValidator) ValidateRequest(req *FetchRequest) error {
	if req == nil {
		return fmt.Errorf("请求参数不能为空")
	}

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

	if !v.validatePlatform(req.Platform) {
		return fmt.Errorf("不支持的平台: %s", req.Platform)
	}

	if !v.validateRegion(req.Region) {
		return fmt.Errorf("不支持的地区: %s", req.Region)
	}

	return v.validateBusinessRules(req)
}

// ValidateProduct 验证产品数据
func (v *ProductValidator) ValidateProduct(product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据不能为空")
	}

	if strings.TrimSpace(product.Asin) == "" {
		return fmt.Errorf("产品ASIN不能为空")
	}

	if strings.TrimSpace(product.Title) == "" {
		return fmt.Errorf("产品标题不能为空")
	}

	if err := v.validatePrice(product); err != nil {
		return err
	}

	return v.validateImages(product)
}

func (v *ProductValidator) validateBusinessRules(req *FetchRequest) error {
	if err := v.validateProductIDFormat(req.Platform, req.ProductID); err != nil {
		return err
	}
	return v.validatePlatformRegionCombination(req.Platform, req.Region)
}

func (v *ProductValidator) validateProductIDFormat(platform, productID string) error {
	switch strings.ToLower(platform) {
	case "amazon":
		if len(productID) != 10 {
			return fmt.Errorf("amazon ASIN必须是10位字符")
		}
		for _, char := range productID {
			if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
				return fmt.Errorf("amazon ASIN只能包含大写字母和数字")
			}
		}
	case "temu":
		if len(productID) == 0 {
			return fmt.Errorf("TEMU产品ID不能为空")
		}
	case "shein":
		if len(productID) == 0 {
			return fmt.Errorf("SHEIN产品ID不能为空")
		}
	}
	return nil
}

func (v *ProductValidator) validatePlatformRegionCombination(platform, region string) error {
	supportedCombinations := map[string][]string{
		"amazon": {"us", "uk", "de", "fr", "it", "es", "ca", "jp", "au", "mx", "ae", "sa"},
		"temu":   {"us", "uk", "de", "fr", "it", "es", "ca", "jp", "au"},
		"shein":  {"us", "uk", "de", "fr", "it", "es"},
	}

	platform = strings.ToLower(platform)
	region = strings.ToLower(region)

	if regions, exists := supportedCombinations[platform]; exists {
		for _, r := range regions {
			if region == r {
				return nil
			}
		}
		return fmt.Errorf("平台 %s 不支持地区 %s", platform, region)
	}
	return fmt.Errorf("不支持的平台: %s", platform)
}

func (v *ProductValidator) validatePrice(product *model.Product) error {
	if product.FinalPrice < 0 {
		return fmt.Errorf("产品价格不能为负数: %f", product.FinalPrice)
	}
	if product.PricesBreakdown.ListPrice != nil && *product.PricesBreakdown.ListPrice < 0 {
		return fmt.Errorf("产品原价不能为负数: %f", *product.PricesBreakdown.ListPrice)
	}
	if product.PricesBreakdown.ListPrice != nil && *product.PricesBreakdown.ListPrice > 0 &&
		product.InitialPrice > product.FinalPrice {
		v.logger.Warnf("产品 %s 当前价格(%f)高于原价(%f)",
			product.Asin, product.FinalPrice, *product.PricesBreakdown.ListPrice)
	}
	return nil
}

func (v *ProductValidator) validateImages(product *model.Product) error {
	if strings.TrimSpace(product.ImageURL) == "" {
		return fmt.Errorf("产品主图不能为空")
	}
	if !strings.HasPrefix(product.ImageURL, "http") {
		return fmt.Errorf("产品主图URL格式不正确: %s", product.ImageURL)
	}
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

func (v *ProductValidator) validatePlatform(platform string) bool {
	platform = strings.ToLower(platform)
	for _, s := range []string{"amazon", "temu", "shein"} {
		if platform == s {
			return true
		}
	}
	return false
}

func (v *ProductValidator) validateRegion(region string) bool {
	region = strings.ToLower(region)
	for _, s := range []string{"us", "uk", "de", "fr", "it", "es", "ca", "jp", "au", "mx", "ae", "sa"} {
		if region == s {
			return true
		}
	}
	return false
}

