package shein

import (
	"fmt"

	"task-processor/internal/common/amazon"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/product"

	"github.com/sirupsen/logrus"
)

// fetchAmazonProduct 从 Amazon 获取产品信息
func fetchAmazonProduct(
	amazonProcessor *amazon.AmazonProcessor,
	asin, region, zipcode string,
) (*model.Product, error) {
	if asin == "" {
		return nil, fmt.Errorf("ASIN 为空")
	}

	domainResolver := product.NewDomainResolver()
	domain := domainResolver.GetAmazonDomainByRegion(region)
	url := fmt.Sprintf("https://%s/dp/%s?th=1&psc=1&language=en_US", domain, asin)

	logger := logrus.WithFields(logrus.Fields{
		"asin":    asin,
		"region":  region,
		"domain":  domain,
		"zipcode": zipcode,
		"url":     url,
	})

	logger.Debug("开始从 Amazon 获取产品信息")

	amazonProduct, err := amazonProcessor.Process(url, zipcode)
	if err != nil {
		return nil, fmt.Errorf("查询 Amazon 产品失败: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"price":        amazonProduct.FinalPrice,
		"is_available": amazonProduct.IsAvailable,
		"availability": amazonProduct.Availability,
	}).Info("成功获取 Amazon 产品信息")

	return amazonProduct, nil
}

// extractStockFromProduct 从 Amazon 产品信息中提取库存数量
func extractStockFromProduct(prod *model.Product) int {
	if !prod.IsAvailable {
		return 0
	}
	if prod.MaxQuantityAvailable > 0 {
		return prod.MaxQuantityAvailable
	}
	return 31
}
