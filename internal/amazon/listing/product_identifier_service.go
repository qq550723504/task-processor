package listing

import (
	"task-processor/internal/amazon/model"

	"github.com/sirupsen/logrus"
)

// ProductIdentifierService 产品标识符服务
type ProductIdentifierService struct {
	logger *logrus.Entry
}

// NewProductIdentifierService 创建产品标识符服务
func NewProductIdentifierService() *ProductIdentifierService {
	return &ProductIdentifierService{
		logger: logrus.WithField("service", "ProductIdentifier"),
	}
}

// BuildIdentifierAttributes 构建产品标识符属性
func (s *ProductIdentifierService) BuildIdentifierAttributes(
	config *model.ProductIdentifierConfig,
	sku string,
	marketplaceID string,
	isAutomotiveCategory bool,
	productType string,
) map[string]any {
	attrs := make(map[string]any)

	// 对于APPAREL类型，不添加任何产品标识符
	if productType == "APPAREL" {
		s.logger.WithField("sku", sku).Info("APPAREL类型跳过产品标识符")
		return attrs
	}

	// 如果没有配置，返回空属性
	if config == nil {
		s.logger.WithField("sku", sku).Info("无产品标识符配置")
		return attrs
	}

	// 构建外部产品ID属性
	if config.UPC != "" {
		attrs["external_product_id"] = []map[string]any{
			{"value": config.UPC, "marketplace_id": marketplaceID},
		}
		attrs["external_product_id_type"] = []map[string]any{
			{"value": "UPC", "marketplace_id": marketplaceID},
		}
		s.logger.WithFields(logrus.Fields{
			"sku": sku,
			"upc": config.UPC,
		}).Info("设置UPC作为外部产品ID")
	} else if config.EAN != "" {
		attrs["external_product_id"] = []map[string]any{
			{"value": config.EAN, "marketplace_id": marketplaceID},
		}
		attrs["external_product_id_type"] = []map[string]any{
			{"value": "EAN", "marketplace_id": marketplaceID},
		}
		s.logger.WithFields(logrus.Fields{
			"sku": sku,
			"ean": config.EAN,
		}).Info("设置EAN作为外部产品ID")
	} else if config.GTIN != "" {
		attrs["external_product_id"] = []map[string]any{
			{"value": config.GTIN, "marketplace_id": marketplaceID},
		}
		attrs["external_product_id_type"] = []map[string]any{
			{"value": "GTIN", "marketplace_id": marketplaceID},
		}
		s.logger.WithFields(logrus.Fields{
			"sku":  sku,
			"gtin": config.GTIN,
		}).Info("设置GTIN作为外部产品ID")
	} else {
		s.logger.WithField("sku", sku).Warn("未提供有效的产品标识符")
	}

	return attrs
}

// GetSupportedIdentifierTypes 获取支持的产品标识符类型（基于亚马逊官方API）
func (s *ProductIdentifierService) GetSupportedIdentifierTypes() []string {
	return []string{
		"ASIN",   // Amazon Standard Identification Number
		"EAN",    // European Article Number
		"FNSKU",  // Fulfillment Network Stock Keeping Unit
		"GTIN",   // Global Trade Item Number
		"ISBN",   // International Standard Book Number
		"JAN",    // Japanese Article Number
		"MINSAN", // Minsan Code
		"SKU",    // Stock Keeping Unit
		"UPC",    // Universal Product Code
	}
}

// GetIdentifierPriority 获取标识符优先级（用于选择最佳标识符）
func (s *ProductIdentifierService) GetIdentifierPriority() map[string]int {
	return map[string]int{
		"UPC":    1, // 最高优先级
		"EAN":    2,
		"GTIN":   3,
		"ISBN":   4,
		"JAN":    5,
		"MINSAN": 6,
		"FNSKU":  7,
		"ASIN":   8,
		"SKU":    9, // 最低优先级
	}
}

