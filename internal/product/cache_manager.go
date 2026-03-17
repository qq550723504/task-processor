// Package product 提供产品数据缓存管理功能
package product

import (
	"encoding/json"
	"fmt"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// CacheManager 缓存管理器
type CacheManager struct {
	rawJsonDataClient RawJsonDataClient
	dataParser        *DataParser
	logger            *logrus.Entry
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(rawJsonDataClient RawJsonDataClient, logger *logrus.Entry) *CacheManager {
	return &CacheManager{
		rawJsonDataClient: rawJsonDataClient,
		dataParser:        NewDataParser(logger),
		logger:            logger.WithField("component", "CacheManager"),
	}
}

// GetFromCache 从缓存获取产品数据
func (c *CacheManager) GetFromCache(req *FetchRequest) (*model.Product, error) {
	apiReq := &RawJsonReq{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		ProductID:  req.ProductID,
		Region:     req.Region,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	}

	rawJsonData, err := c.rawJsonDataClient.GetRawJsonData(apiReq)
	if err != nil {
		return nil, fmt.Errorf("获取缓存数据失败: %w", err)
	}
	if rawJsonData == nil || rawJsonData.RawJSONData == "" {
		return nil, fmt.Errorf("缓存数据为空")
	}

	c.logger.Infof("✅ 服务器有历史数据: ProductID=%s, 数据长度=%d", req.ProductID, len(rawJsonData.RawJSONData))

	product, parseErr := c.dataParser.ParseAmazonProduct(rawJsonData.RawJSONData)
	if parseErr != nil {
		return nil, fmt.Errorf("解析缓存数据失败: %w", parseErr)
	}
	if c.needsRefetch(product) {
		return nil, fmt.Errorf("缓存数据需要更新")
	}
	return product, nil
}

// SaveToCache 保存产品数据到缓存
func (c *CacheManager) SaveToCache(req *FetchRequest, product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据为空")
	}
	jsonData, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	createReq := &RawJsonCreateReq{
		TenantID:    req.TenantID,
		Platform:    req.Platform,
		Region:      req.Region,
		ProductID:   req.ProductID,
		RawJsonData: string(jsonData),
		Creator:     req.Creator,
		StoreID:     req.StoreID,
		CategoryID:  req.CategoryID,
	}
	id, err := c.rawJsonDataClient.CreateRawJsonData(createReq)
	if err != nil {
		return fmt.Errorf("保存失败: %w", err)
	}
	c.logger.Infof("✅ 保存成功: ProductID=%s, ID=%d", req.ProductID, id)
	return nil
}

// CacheProduct 缓存产品数据（检查是否已存在）
func (c *CacheManager) CacheProduct(req *FetchRequest, product *model.Product) error {
	if _, err := c.GetFromCache(req); err == nil {
		c.logger.Infof("⏭️ 服务器已有产品数据缓存，跳过: ProductID=%s", req.ProductID)
		return nil
	}
	return c.SaveToCache(req, product)
}

// CacheVariants 批量缓存变体数据
func (c *CacheManager) CacheVariants(req *FetchRequest, variants []*model.Product) error {
	successCount, failCount, skipCount := 0, 0, 0
	for _, variant := range variants {
		if variant == nil {
			skipCount++
			continue
		}
		variantReq := &FetchRequest{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			Region:     req.Region,
			ProductID:  variant.Asin,
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		}
		if _, err := c.GetFromCache(variantReq); err == nil {
			skipCount++
			continue
		}
		if saveErr := c.SaveToCache(variantReq, variant); saveErr != nil {
			c.logger.Errorf("保存变体数据失败 (ASIN: %s): %v", variant.Asin, saveErr)
			failCount++
			continue
		}
		successCount++
	}
	c.logger.Infof("✅ 变体数据缓存完成: 成功=%d, 失败=%d, 跳过=%d, 总数=%d",
		successCount, failCount, skipCount, len(variants))
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("所有变体数据缓存失败: 失败数=%d", failCount)
	}
	return nil
}

func (c *CacheManager) needsRefetch(product *model.Product) bool {
	if c.needsRefetchForOldFormat(product) {
		c.logger.Warnf("⚠️ 检测到旧版数据格式（variations 缺少 attributes），需要重新抓取")
		return true
	}
	if c.needsRefetchForMissingShipsFrom(product) {
		c.logger.Warnf("⚠️ 检测到缓存数据缺少 ShipsFrom 字段，需要重新抓取")
		return true
	}
	return false
}

func (c *CacheManager) needsRefetchForOldFormat(product *model.Product) bool {
	if product == nil || len(product.Variations) == 0 {
		return false
	}
	for _, variation := range product.Variations {
		if len(variation.Attributes) == 0 {
			return true
		}
	}
	return false
}

func (c *CacheManager) needsRefetchForMissingShipsFrom(product *model.Product) bool {
	return product != nil && product.ShipsFrom == ""
}

