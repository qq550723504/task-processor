// Package service 提供Amazon产品标识符豁免业务逻辑
package service

import (
	"task-processor/internal/amazon/model"

	"github.com/sirupsen/logrus"
)

// ExemptionService 豁免服务
type ExemptionService struct {
	logger *logrus.Entry
}

// NewExemptionService 创建豁免服务
func NewExemptionService() *ExemptionService {
	return &ExemptionService{
		logger: logrus.WithField("service", "Exemption"),
	}
}

// ExemptionReason 豁免原因枚举
type ExemptionReason string

const (
	// ExemptionReasonGeneric 通用产品（无品牌标识符）
	ExemptionReasonGeneric ExemptionReason = "generic_product"
	// ExemptionReasonPrivateLabel 自有品牌产品
	ExemptionReasonPrivateLabel ExemptionReason = "private_label"
	// ExemptionReasonBundle 产品组合包
	ExemptionReasonBundle ExemptionReason = "bundle"
	// ExemptionReasonAutomotive 汽车配件（符合Amazon政策）
	ExemptionReasonAutomotive ExemptionReason = "automotive_parts"
	// ExemptionReasonHandmade 手工制品
	ExemptionReasonHandmade ExemptionReason = "handmade"
)

// GetExemptionReason 根据产品类型和配置获取豁免原因
func (s *ExemptionService) GetExemptionReason(
	productType string,
	config *model.ProductIdentifierConfig,
	isAutomotiveCategory bool,
) ExemptionReason {
	// 汽配类目自动使用汽配豁免
	if isAutomotiveCategory {
		return ExemptionReasonAutomotive
	}

	// 根据产品类型判断
	switch productType {
	case "HANDMADE":
		return ExemptionReasonHandmade
	case "BUNDLE":
		return ExemptionReasonBundle
	default:
		// 默认使用通用产品豁免
		return ExemptionReasonGeneric
	}
}

// BuildExemptionAttributes 构建完整的豁免属性
func (s *ExemptionService) BuildExemptionAttributes(
	reason ExemptionReason,
	sku string,
	marketplaceID string,
) map[string]any {
	attrs := make(map[string]any)

	// 根据Amazon官方文档深入研究，正确的做法是：
	// 1. 完全不设置 externally_assigned_product_identifier
	// 2. 不设置任何豁免相关属性（merchant_suggested_asin, item_type_keyword等）
	// 3. 让Amazon根据产品类型Schema和缺少外部标识符自动处理
	// 4. Amazon后台的"外部产品ID"要求是Schema级别的，不是API级别的

	s.logger.WithFields(logrus.Fields{
		"sku":    sku,
		"reason": string(reason),
	}).Info("使用标准方式处理产品标识符，完全不设置豁免相关属性")

	return attrs
}

// addReasonSpecificAttributes 根据豁免原因添加特定属性
func (s *ExemptionService) addReasonSpecificAttributes(
	attrs map[string]any,
	reason ExemptionReason,
	marketplaceID string,
) {
	// 简化实现，避免添加可能冲突的属性
	// 豁免声明应该尽可能简洁，只包含必需的属性
	switch reason {
	case ExemptionReasonAutomotive:
		// 汽配产品只添加最基本的属性
		// 其他属性由动态模板根据Schema自动处理
	case ExemptionReasonBundle:
		// 组合产品的基本属性
	case ExemptionReasonHandmade:
		// 手工制品的基本属性
	default:
		// 通用产品不添加额外属性
	}
}

// generateMerchantSuggestedASIN 生成商家建议的ASIN
func (s *ExemptionService) generateMerchantSuggestedASIN(sku string) string {
	// 基于SKU生成一个符合ASIN格式的标识符
	// ASIN格式：B + 9位字母数字组合
	hash := 0
	for _, char := range sku {
		hash = (hash*31 + int(char)) % 1000000000
	}

	// 生成9位字母数字组合
	chars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := "B"

	for i := 0; i < 9; i++ {
		result += string(chars[hash%len(chars)])
		hash = hash / len(chars)
		if hash == 0 {
			hash = int(sku[i%len(sku)]) + 1
		}
	}

	return result
}

// ValidateExemptionReason 验证豁免原因是否有效
func (s *ExemptionService) ValidateExemptionReason(reason ExemptionReason) bool {
	validReasons := map[ExemptionReason]bool{
		ExemptionReasonGeneric:      true,
		ExemptionReasonPrivateLabel: true,
		ExemptionReasonBundle:       true,
		ExemptionReasonAutomotive:   true,
		ExemptionReasonHandmade:     true,
	}

	return validReasons[reason]
}

// GetReasonDescription 获取豁免原因的描述
func (s *ExemptionService) GetReasonDescription(reason ExemptionReason) string {
	descriptions := map[ExemptionReason]string{
		ExemptionReasonGeneric:      "通用产品，无外部产品标识符",
		ExemptionReasonPrivateLabel: "自有品牌产品",
		ExemptionReasonBundle:       "产品组合包",
		ExemptionReasonAutomotive:   "汽车配件，符合Amazon GTIN豁免政策",
		ExemptionReasonHandmade:     "手工制品",
	}

	if desc, ok := descriptions[reason]; ok {
		return desc
	}
	return "未知豁免原因"
}

// getGTINExemptionReason 获取GTIN豁免原因的Amazon标准值
func (s *ExemptionService) getGTINExemptionReason(reason ExemptionReason) string {
	// Amazon标准的GTIN豁免原因值
	gtinReasons := map[ExemptionReason]string{
		ExemptionReasonGeneric:      "product_not_sold_in_retail",
		ExemptionReasonPrivateLabel: "private_label",
		ExemptionReasonBundle:       "bundle",
		ExemptionReasonAutomotive:   "automotive_parts", // 汽配类目的正确值
		ExemptionReasonHandmade:     "handmade",
	}

	if gtinReason, ok := gtinReasons[reason]; ok {
		return gtinReason
	}
	return "product_not_sold_in_retail"
}
