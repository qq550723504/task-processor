// Package sync 提供SHEIN平台活动产品数据转换功能
package sync

import (
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// ActivityConverter 活动产品数据转换器
type ActivityConverter struct {
	logger *logrus.Entry
}

// NewActivityConverter 创建新的活动产品转换器
func NewActivityConverter() *ActivityConverter {
	return &ActivityConverter{
		logger: logrus.WithField("component", "ActivityConverter"),
	}
}

// ConvertToBackendFormat 转换为后端API格式
func (c *ActivityConverter) ConvertToBackendFormat(
	products []marketing.SkcInfo,
	tenantID, storeID int64,
) []api.ActivityProductDTO {
	backendProducts := make([]api.ActivityProductDTO, 0, len(products))

	for _, product := range products {
		backendProduct := c.convertSingleProduct(&product, tenantID, storeID)
		backendProducts = append(backendProducts, backendProduct)
	}

	c.logger.WithFields(logrus.Fields{
		"store_id": storeID,
		"count":    len(backendProducts),
	}).Debug("成功转换活动产品数据")

	return backendProducts
}

// convertSingleProduct 转换单个活动产品
func (c *ActivityConverter) convertSingleProduct(
	product *marketing.SkcInfo,
	tenantID, storeID int64,
) api.ActivityProductDTO {
	// 转换 SkcInfo 为 ActivityProductDTO
	return api.ActivityProductDTO{
		Platform:            "SHEIN",
		TenantID:            tenantID,
		StoreID:             storeID,
		SKC:                 product.Skc,
		GoodsName:           product.GoodsName,
		Image:               product.Image,
		SupplierNo:          product.SupplierNo,
		Stock:               product.Stock,
		SupplyPrice:         product.SupplyPrice,
		SupplyPriceCurrency: product.SupplyPriceCurrency,
		IsConfigured:        product.IsConfigured,
	}
}
