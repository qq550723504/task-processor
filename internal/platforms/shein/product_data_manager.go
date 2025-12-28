// Package shein 提供SHEIN产品数据管理功能
package shein

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/common/amazon"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management/api"
	"task-processor/internal/platforms/shein/modules"

	"github.com/sirupsen/logrus"
)

// ProductDataManager 产品数据管理器
type ProductDataManager struct {
	amazonProcessor       *amazon.AmazonProcessor
	rawJsonDataClient     api.RawJsonDataAPI
	inventoryRecordClient api.InventoryRecordAPI
	syncService           *SyncService
}

// NewProductDataManager 创建产品数据管理器
func NewProductDataManager(
	amazonProcessor *amazon.AmazonProcessor,
	rawJsonDataClient api.RawJsonDataAPI,
	inventoryRecordClient api.InventoryRecordAPI,
	syncService *SyncService,
) *ProductDataManager {
	return &ProductDataManager{
		amazonProcessor:       amazonProcessor,
		rawJsonDataClient:     rawJsonDataClient,
		inventoryRecordClient: inventoryRecordClient,
		syncService:           syncService,
	}
}

// GetStoreMappings 获取店铺的所有产品数据
func (m *ProductDataManager) GetStoreMappings(storeID, tenantID int64) ([]*api.ProductDataDTO, error) {
	shelfStatus := api.ShelfStatusOnShelf
	productDataClient := m.syncService.repositoryFactory(storeID, tenantID)
	products, err := productDataClient.ListByStore("SHEIN", tenantID, storeID, &shelfStatus)
	if err != nil {
		return nil, fmt.Errorf("查询店铺产品列表失败: %w", err)
	}
	return products, nil
}

// GetAmazonProductData 获取 Amazon 产品数据（先查数据库，没有再爬取）
func (m *ProductDataManager) GetAmazonProductData(asin string, mappingInfo *MappingInfo, tenantID, storeID int64, priceType string) (*model.Product, error) {
	region := mappingInfo.Region
	if region == "" {
		region = "US"
	}

	zipcode := m.getDefaultZipcode(region)

	// 先从数据库查询
	amazonProduct, err := m.getFromDatabase(asin, region, tenantID, storeID)
	if err == nil && amazonProduct != nil {
		return amazonProduct, nil
	}

	// 数据库中没有数据、数据过期或解析失败，从 Amazon 爬取
	logrus.WithFields(logrus.Fields{
		"asin":   asin,
		"region": region,
	}).Debug("从 Amazon 爬取产品数据")

	amazonProduct, err = fetchAmazonProduct(m.amazonProcessor, asin, region, zipcode)
	if err != nil {
		return nil, err
	}

	// 爬取成功后保存到数据库
	go m.saveAmazonProductData(asin, region, amazonProduct, tenantID, storeID)

	// 记录库存和价格变动（每天一次）
	go m.recordInventoryAndPrice(asin, region, amazonProduct, priceType)

	return amazonProduct, nil
}

// getFromDatabase 从数据库获取Amazon产品数据
func (m *ProductDataManager) getFromDatabase(asin, region string, tenantID, storeID int64) (*model.Product, error) {
	req := &api.RawJsonDataReqDTO{
		TenantID:   tenantID,
		Platform:   "Amazon",
		Region:     region,
		ProductID:  asin,
		StoreID:    storeID,
		CategoryID: 0,
		Creator:    "monitor",
	}

	rawData, err := m.rawJsonDataClient.GetRawJsonData(req)
	if err != nil || rawData == nil || rawData.RawJSONData == "" {
		return nil, fmt.Errorf("数据库中没有数据")
	}

	// 检查数据新鲜度（24小时内）
	timestamp := rawData.UpdateTime
	if timestamp == 0 {
		timestamp = rawData.CreateTime
	}
	dataAge := time.Now().Unix() - timestamp/1000 // 时间戳是毫秒
	if dataAge >= 24*3600 {
		logrus.WithFields(logrus.Fields{
			"asin":      asin,
			"region":    region,
			"age_hours": dataAge / 3600,
		}).Debug("数据库中的数据已过期（超过24小时），重新爬取")
		return nil, fmt.Errorf("数据已过期")
	}

	// 数据在24小时内，解析并返回
	var amazonProduct model.Product
	parseErr := json.Unmarshal([]byte(rawData.RawJSONData), &amazonProduct)
	if parseErr != nil {
		logrus.WithError(parseErr).Warn("解析数据库中的 Amazon 数据失败，尝试重新爬取")
		return nil, parseErr
	}

	logrus.WithFields(logrus.Fields{
		"asin":      asin,
		"region":    region,
		"age_hours": dataAge / 3600,
	}).Debug("从数据库获取 Amazon 产品数据")

	return &amazonProduct, nil
}

// saveAmazonProductData 保存 Amazon 产品数据到数据库（异步）
func (m *ProductDataManager) saveAmazonProductData(asin, region string, amazonProduct *model.Product, tenantID, storeID int64) {
	rawJSON, err := json.Marshal(amazonProduct)
	if err != nil {
		logrus.WithError(err).WithField("asin", asin).Error("序列化 Amazon 产品数据失败")
		return
	}

	createReq := &api.RawJsonDataCreateReqDTO{
		TenantID:    tenantID,
		Platform:    "Amazon",
		Region:      region,
		ProductID:   asin,
		RawJsonData: string(rawJSON),
		StoreID:     storeID,
		Creator:     "monitor",
	}

	_, err = m.rawJsonDataClient.CreateRawJsonData(createReq)
	if err != nil {
		logrus.WithError(err).WithField("asin", asin).Error("保存 Amazon 产品数据到数据库失败")
	} else {
		logrus.WithField("asin", asin).Debug("已保存 Amazon 产品数据到数据库")
	}
}

// UpdateAttributesWithAmazonData 更新产品 attributes 中的 Amazon 数据（异步）
func (m *ProductDataManager) UpdateAttributesWithAmazonData(prod *api.ProductDataDTO, platformSKU string, amazonProduct *model.Product, tenantID, storeID int64, priceType string) {
	// 解析现有的 attributes
	var skcList []SKCInfo
	if err := json.Unmarshal([]byte(prod.Attributes), &skcList); err != nil {
		logrus.WithError(err).WithField("product_id", prod.ProductID).Error("解析产品 attributes 失败")
		return
	}

	// 查找对应的 SKU 并添加 Amazon 监控数据
	updated := false
	for i := range skcList {
		for j := range skcList[i].SKUInfo {
			sku := &skcList[i].SKUInfo[j]
			if sku.MappingInfo != nil && sku.MappingInfo.SKU == platformSKU {
				// 创建或更新 Amazon 监控数据（新增字段）
				newStock := extractStockFromProduct(amazonProduct)
				sku.AmazonMonitorData = &AmazonMonitorData{
					ASIN:          sku.MappingInfo.ProductID,
					Price:         modules.GetProductPrice(amazonProduct, priceType),
					Stock:         newStock,
					LastCheckTime: time.Now().Unix(),
				}

				updated = true
				logrus.WithFields(logrus.Fields{
					"platform_sku": platformSKU,
					"asin":         sku.MappingInfo.ProductID,
					"price":        sku.AmazonMonitorData.Price,
					"stock":        newStock,
				}).Debug("添加 SKU 的 Amazon 监控数据")
				break
			}
		}
		if updated {
			break
		}
	}

	if !updated {
		logrus.WithField("platform_sku", platformSKU).Debug("未找到对应的 SKU，跳过更新")
		return
	}

	// 序列化更新后的 attributes
	updatedAttributes, err := json.Marshal(skcList)
	if err != nil {
		logrus.WithError(err).Error("序列化更新后的 attributes 失败")
		return
	}

	// 更新产品数据
	prod.Attributes = string(updatedAttributes)

	// 保存到数据库
	productDataClient := m.syncService.repositoryFactory(storeID, tenantID)
	if err := productDataClient.CreateOrUpdate(prod); err != nil {
		logrus.WithError(err).WithField("product_id", prod.ProductID).Error("保存更新后的产品数据失败")
	} else {
		logrus.WithField("product_id", prod.ProductID).Debug("已添加产品 attributes 中的 Amazon 监控数据")
	}
}

// recordInventoryAndPrice 记录库存和价格变动（每天一次）
func (m *ProductDataManager) recordInventoryAndPrice(productId, region string, amazonProduct *model.Product, priceType string) {
	// 检查今天是否已经记录过
	latestRecord, err := m.inventoryRecordClient.GetLatestInventoryRecord("Amazon", productId, region)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"productId": productId,
			"region":    region,
		}).Warn("查询最新库存记录失败")
	}

	// 如果今天已经记录过，跳过
	if latestRecord != nil {
		recordDate := latestRecord.CreateTime.Format("2006-01-02")
		today := time.Now().Format("2006-01-02")
		if recordDate == today {
			logrus.WithFields(logrus.Fields{
				"product_id": productId,
				"region":     region,
			}).Debug("今天已记录过库存和价格，跳过")
			return
		}
	}

	// 提取库存和价格信息
	stock := extractStockFromProduct(amazonProduct)

	// 获取原价（从 prices_breakdown.list_price 或 initial_price）
	var originalPrice float64
	if amazonProduct.PricesBreakdown.ListPrice != nil {
		originalPrice = *amazonProduct.PricesBreakdown.ListPrice
	} else {
		originalPrice = amazonProduct.InitialPrice
	}

	// 根据店铺配置获取当前价格
	currentPrice := modules.GetProductPrice(amazonProduct, priceType)
	currency := amazonProduct.Currency

	// 计算价格变化百分比
	var priceChangePercent *float64
	if latestRecord != nil && latestRecord.CurrentPrice != nil && *latestRecord.CurrentPrice > 0 {
		change := ((currentPrice - *latestRecord.CurrentPrice) / *latestRecord.CurrentPrice) * 100
		priceChangePercent = &change
	}

	// 创建库存记录
	recordReq := &api.InventoryRecordCreateReqDTO{
		Platform:           "Amazon",
		ProductId:          productId,
		Region:             region,
		Stock:              &stock,
		StockStatus:        amazonProduct.Availability,
		IsAvailable:        amazonProduct.IsAvailable,
		OriginalPrice:      &originalPrice,
		CurrentPrice:       &currentPrice,
		Currency:           currency,
		PriceChangePercent: priceChangePercent,
		SyncSource:         "MONITOR",
		Remark:             "产品监控自动记录",
	}

	recordID, err := m.inventoryRecordClient.CreateInventoryRecord(recordReq)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"product_id": productId,
			"region":     region,
		}).Error("创建库存价格记录失败")
	} else {
		logrus.WithFields(logrus.Fields{
			"product_id": productId,
			"region":     region,
			"record_id":  recordID,
			"stock":      stock,
			"price":      currentPrice,
		}).Info("✅ 已记录库存和价格变动")
	}
}

// getDefaultZipcode 获取默认邮编
func (m *ProductDataManager) getDefaultZipcode(region string) string {
	// 使用默认邮编映射
	defaultZipcodes := map[string]string{
		"us": "10001",     // 纽约
		"ca": "M5V 3A8",   // 多伦多
		"uk": "SW1A 1AA",  // 伦敦
		"de": "10115",     // 柏林
		"fr": "75001",     // 巴黎
		"it": "00118",     // 罗马
		"es": "28001",     // 马德里
		"jp": "100-0001",  // 东京
		"au": "2000",      // 悉尼
		"in": "110001",    // 新德里
		"mx": "01000",     // 墨西哥城
		"br": "01310-100", // 圣保罗
	}

	if zipcode, exists := defaultZipcodes[strings.ToLower(region)]; exists {
		return zipcode
	}

	// 默认返回美国邮编
	return "10001"
}
