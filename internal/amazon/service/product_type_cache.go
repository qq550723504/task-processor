// Package service 提供Amazon产品类型缓存服务
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// ProductTypeCache 产品类型缓存
type ProductTypeCache struct {
	apiClient   *api.Client
	cacheDir    string
	logger      *logrus.Entry
	cache       map[string]*MarketplaceProductTypes // marketplaceID -> 产品类型列表
	cacheMutex  sync.RWMutex
	cacheExpiry time.Duration
}

// MarketplaceProductTypes 站点产品类型数据
type MarketplaceProductTypes struct {
	MarketplaceID string               `json:"marketplace_id"`
	ProductTypes  []ProductTypeSummary `json:"product_types"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Count         int                  `json:"count"`
}

// ProductTypeSummary 产品类型摘要（本地缓存结构）
type ProductTypeSummary struct {
	ProductType string `json:"product_type"`
	DisplayName string `json:"display_name"`
}

// NewProductTypeCache 创建产品类型缓存服务
func NewProductTypeCache(apiClient *api.Client, cacheDir string) *ProductTypeCache {
	if cacheDir == "" {
		cacheDir = "cache/product_types"
	}

	return &ProductTypeCache{
		apiClient:   apiClient,
		cacheDir:    cacheDir,
		logger:      logrus.WithField("service", "ProductTypeCache"),
		cache:       make(map[string]*MarketplaceProductTypes),
		cacheExpiry: 24 * time.Hour, // 默认24小时过期
	}
}

// SetCacheExpiry 设置缓存过期时间
func (c *ProductTypeCache) SetCacheExpiry(expiry time.Duration) {
	c.cacheExpiry = expiry
}

// GetAllProductTypes 获取所有产品类型（优先从缓存读取）
func (c *ProductTypeCache) GetAllProductTypes(ctx context.Context, marketplaceID string) (*MarketplaceProductTypes, error) {
	// 1. 先检查内存缓存
	if cached := c.getFromMemory(marketplaceID); cached != nil {
		c.logger.Debugf("从内存缓存获取产品类型: marketplace=%s, count=%d", marketplaceID, cached.Count)
		return cached, nil
	}

	// 2. 检查文件缓存
	if cached, err := c.loadFromFile(marketplaceID); err == nil && cached != nil {
		if !c.isExpired(cached) {
			c.setToMemory(marketplaceID, cached)
			c.logger.Infof("从文件缓存加载产品类型: marketplace=%s, count=%d", marketplaceID, cached.Count)
			return cached, nil
		}
		c.logger.Info("文件缓存已过期，需要刷新")
	}

	// 3. 从API获取并缓存
	return c.refreshCache(ctx, marketplaceID)
}

// RefreshCache 强制刷新缓存
func (c *ProductTypeCache) RefreshCache(ctx context.Context, marketplaceID string) (*MarketplaceProductTypes, error) {
	return c.refreshCache(ctx, marketplaceID)
}

// refreshCache 从API刷新缓存
func (c *ProductTypeCache) refreshCache(ctx context.Context, marketplaceID string) (*MarketplaceProductTypes, error) {
	c.logger.Infof("从API获取产品类型列表: marketplace=%s", marketplaceID)

	// 调用API获取所有产品类型（不传keywords）
	result, err := c.apiClient.SearchProductTypes(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("获取产品类型列表失败: %w", err)
	}

	// 转换为本地结构
	// 注意：Amazon API 有时返回的 productType 为空，此时使用 displayName 作为 productType
	productTypes := make([]ProductTypeSummary, 0, len(result))
	for _, pt := range result {
		productType := pt.Name
		if productType == "" {
			productType = pt.DisplayName
		}
		productTypes = append(productTypes, ProductTypeSummary{
			ProductType: productType,
			DisplayName: pt.DisplayName,
		})
	}

	cached := &MarketplaceProductTypes{
		MarketplaceID: marketplaceID,
		ProductTypes:  productTypes,
		UpdatedAt:     time.Now(),
		Count:         len(productTypes),
	}

	// 保存到内存和文件
	c.setToMemory(marketplaceID, cached)
	if err := c.saveToFile(marketplaceID, cached); err != nil {
		c.logger.Warnf("保存缓存文件失败: %v", err)
	}

	c.logger.Infof("产品类型缓存已更新: marketplace=%s, count=%d", marketplaceID, cached.Count)
	return cached, nil
}

// SearchByKeyword 根据关键词搜索产品类型（从缓存中搜索）
func (c *ProductTypeCache) SearchByKeyword(ctx context.Context, marketplaceID, keyword string, limit int) ([]ProductTypeSummary, error) {
	cached, err := c.GetAllProductTypes(ctx, marketplaceID)
	if err != nil {
		return nil, err
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		// 返回全部（限制数量）
		if limit > 0 && limit < len(cached.ProductTypes) {
			return cached.ProductTypes[:limit], nil
		}
		return cached.ProductTypes, nil
	}

	var results []ProductTypeSummary
	for _, pt := range cached.ProductTypes {
		// 匹配 ProductType 或 DisplayName
		if strings.Contains(strings.ToLower(pt.ProductType), keyword) ||
			strings.Contains(strings.ToLower(pt.DisplayName), keyword) {
			results = append(results, pt)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// GetByProductType 根据产品类型名称获取详情（大小写不敏感）
func (c *ProductTypeCache) GetByProductType(ctx context.Context, marketplaceID, productType string) (*ProductTypeSummary, error) {
	cached, err := c.GetAllProductTypes(ctx, marketplaceID)
	if err != nil {
		return nil, err
	}

	productType = strings.TrimSpace(productType)
	productTypeLower := strings.ToLower(productType)
	for _, pt := range cached.ProductTypes {
		if strings.ToLower(pt.ProductType) == productTypeLower {
			return &pt, nil
		}
	}

	return nil, fmt.Errorf("产品类型不存在: %s", productType)
}

// IsValidProductType 验证产品类型是否有效
func (c *ProductTypeCache) IsValidProductType(ctx context.Context, marketplaceID, productType string) bool {
	pt, err := c.GetByProductType(ctx, marketplaceID, productType)
	return err == nil && pt != nil
}

// getFromMemory 从内存缓存获取
func (c *ProductTypeCache) getFromMemory(marketplaceID string) *MarketplaceProductTypes {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	cached, exists := c.cache[marketplaceID]
	if !exists {
		return nil
	}

	// 检查是否过期
	if c.isExpired(cached) {
		return nil
	}

	return cached
}

// setToMemory 设置内存缓存
func (c *ProductTypeCache) setToMemory(marketplaceID string, data *MarketplaceProductTypes) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cache[marketplaceID] = data
}

// isExpired 检查缓存是否过期
func (c *ProductTypeCache) isExpired(cached *MarketplaceProductTypes) bool {
	return time.Since(cached.UpdatedAt) > c.cacheExpiry
}

// getCacheFilePath 获取缓存文件路径
func (c *ProductTypeCache) getCacheFilePath(marketplaceID string) string {
	filename := fmt.Sprintf("product_types_%s.json", marketplaceID)
	return filepath.Join(c.cacheDir, filename)
}

// loadFromFile 从文件加载缓存
func (c *ProductTypeCache) loadFromFile(marketplaceID string) (*MarketplaceProductTypes, error) {
	filePath := c.getCacheFilePath(marketplaceID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取缓存文件失败: %w", err)
	}

	var cached MarketplaceProductTypes
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("解析缓存文件失败: %w", err)
	}

	return &cached, nil
}

// saveToFile 保存缓存到文件
func (c *ProductTypeCache) saveToFile(marketplaceID string, data *MarketplaceProductTypes) error {
	// 确保目录存在
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	filePath := c.getCacheFilePath(marketplaceID)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化缓存数据失败: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入缓存文件失败: %w", err)
	}

	c.logger.Infof("缓存已保存到文件: %s", filePath)
	return nil
}

// ClearCache 清除缓存
func (c *ProductTypeCache) ClearCache(marketplaceID string) error {
	// 清除内存缓存
	c.cacheMutex.Lock()
	delete(c.cache, marketplaceID)
	c.cacheMutex.Unlock()

	// 删除文件缓存
	filePath := c.getCacheFilePath(marketplaceID)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除缓存文件失败: %w", err)
	}

	c.logger.Infof("缓存已清除: marketplace=%s", marketplaceID)
	return nil
}
