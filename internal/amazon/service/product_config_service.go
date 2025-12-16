// Package service 提供Amazon产品配置业务逻辑
package service

import (
	"task-processor/internal/amazon/model"

	"github.com/sirupsen/logrus"
)

// ProductConfigService 产品配置服务
type ProductConfigService struct {
	logger *logrus.Entry
}

// NewProductConfigService 创建产品配置服务
func NewProductConfigService() *ProductConfigService {
	return &ProductConfigService{
		logger: logrus.WithField("service", "ProductConfig"),
	}
}

// CreateDefaultConfig 创建默认产品配置
func (s *ProductConfigService) CreateDefaultConfig() *model.ProductIdentifierConfig {
	return &model.ProductIdentifierConfig{
		UPC:  "",
		EAN:  "",
		GTIN: "",
	}
}

// CreateConfigWithUPC 创建带UPC的产品配置
func (s *ProductConfigService) CreateConfigWithUPC(upc string) *model.ProductIdentifierConfig {
	return &model.ProductIdentifierConfig{
		UPC:  upc,
		EAN:  "",
		GTIN: "",
	}
}

// CreateConfigWithEAN 创建带EAN的产品配置
func (s *ProductConfigService) CreateConfigWithEAN(ean string) *model.ProductIdentifierConfig {
	return &model.ProductIdentifierConfig{
		UPC:  "",
		EAN:  ean,
		GTIN: "",
	}
}

// CreateConfigWithGTIN 创建带GTIN的产品配置
func (s *ProductConfigService) CreateConfigWithGTIN(gtin string) *model.ProductIdentifierConfig {
	return &model.ProductIdentifierConfig{
		UPC:  "",
		EAN:  "",
		GTIN: gtin,
	}
}

// LogConfigStatus 记录配置状态
func (s *ProductConfigService) LogConfigStatus(config *model.ProductIdentifierConfig, sku string) {
	if !config.HasAnyIdentifier() {
		s.logger.WithField("sku", sku).Warn("此商品没有商品编码，将声明产品标识符豁免")
	} else {
		s.logger.WithField("sku", sku).Info("商品已配置外部产品标识符")

		if config.UPC != "" {
			s.logger.WithField("sku", sku).WithField("upc", config.UPC).Debug("使用UPC码")
		}
		if config.EAN != "" {
			s.logger.WithField("sku", sku).WithField("ean", config.EAN).Debug("使用EAN码")
		}
		if config.GTIN != "" {
			s.logger.WithField("sku", sku).WithField("gtin", config.GTIN).Debug("使用GTIN码")
		}
	}
}
