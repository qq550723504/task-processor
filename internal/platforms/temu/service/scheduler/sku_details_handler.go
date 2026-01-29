// Package scheduler 提供TEMU平台SKU详细信息处理功能
package scheduler

import (
	"encoding/json"
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/types"
	"task-processor/internal/platforms/temu/api/models"
)

// fetchSkuDetails 获取SKU详细信息
func (s *productSyncServiceImpl) fetchSkuDetails(temuProduct *models.GoodsSearchItem) (*models.SkuQueryResponse, error) {
	// 检查是否有必要的参数
	if temuProduct.GoodsCommitID == "" || temuProduct.GoodsID == "" {
		return nil, fmt.Errorf("缺少必要的参数: goods_commit_id=%s, goods_id=%s",
			temuProduct.GoodsCommitID, temuProduct.GoodsID)
	}

	// 调用SKU查询接口
	skuResponse, err := s.skuQueryAPI.QuerySkuPriceAndStock(
		temuProduct.GoodsCommitID,
		temuProduct.GoodsID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询SKU详细信息失败: %w", err)
	}

	return skuResponse, nil
}

// fillSkuBasedPriceAndStockInfo 基于SKU查询结果填充价格和库存信息
func (s *productSyncServiceImpl) fillSkuBasedPriceAndStockInfo(
	productData *managementapi.ProductDataDTO,
	skuDetails *models.SkuQueryResponse,
) {
	if skuDetails == nil || len(skuDetails.Result.SkuList) == 0 {
		s.logger.Warn("SKU详细信息为空，无法填充价格和库存")
		return
	}

	skuList := skuDetails.Result.SkuList

	// 计算价格统计信息
	priceStats := s.calculatePriceStats(skuList)
	stockStats := s.calculateStockStats(skuList)

	// 填充价格信息（使用最低价格作为原价，最高价格作为特价）
	productData.OriginalPrice = types.FlexibleString(fmt.Sprintf("%.2f", priceStats.MinPrice))
	if priceStats.MaxPrice != priceStats.MinPrice {
		productData.SpecialPrice = types.FlexibleString(fmt.Sprintf("%.2f", priceStats.MaxPrice))
	} else {
		productData.SpecialPrice = productData.OriginalPrice
	}

	// 填充库存信息（使用总库存）
	productData.Stock = types.FlexibleString(fmt.Sprintf("%d", stockStats.TotalStock))
}

// PriceStats 价格统计信息
type PriceStats struct {
	MinPrice float64
	MaxPrice float64
	AvgPrice float64
}

// StockStats 库存统计信息
type StockStats struct {
	TotalStock      int
	MinStock        int
	MaxStock        int
	OutOfStockCount int
}

// calculatePriceStats 计算价格统计信息
func (s *productSyncServiceImpl) calculatePriceStats(skuList []models.SkuQueryResultItem) PriceStats {
	if len(skuList) == 0 {
		return PriceStats{}
	}

	stats := PriceStats{
		MinPrice: skuList[0].Price,
		MaxPrice: skuList[0].Price,
	}

	var totalPrice float64
	for _, sku := range skuList {
		if sku.Price < stats.MinPrice {
			stats.MinPrice = sku.Price
		}
		if sku.Price > stats.MaxPrice {
			stats.MaxPrice = sku.Price
		}
		totalPrice += sku.Price
	}

	stats.AvgPrice = totalPrice / float64(len(skuList))
	return stats
}

// calculateStockStats 计算库存统计信息
func (s *productSyncServiceImpl) calculateStockStats(skuList []models.SkuQueryResultItem) StockStats {
	if len(skuList) == 0 {
		return StockStats{}
	}

	stats := StockStats{
		MinStock: skuList[0].Stock,
		MaxStock: skuList[0].Stock,
	}

	for _, sku := range skuList {
		stats.TotalStock += sku.Stock

		if sku.Stock < stats.MinStock {
			stats.MinStock = sku.Stock
		}
		if sku.Stock > stats.MaxStock {
			stats.MaxStock = sku.Stock
		}
		if sku.Stock == 0 {
			stats.OutOfStockCount++
		}
	}

	return stats
}

// fillMappingAttributesWithSkuDetails 使用SKU详细信息填充映射属性数据
func (s *productSyncServiceImpl) fillMappingAttributesWithSkuDetails(
	productData *managementapi.ProductDataDTO,
	temuProduct *models.GoodsSearchItem,
	skuDetails *models.SkuQueryResponse,
	storeID int64,
) error {
	if skuDetails == nil || len(skuDetails.Result.SkuList) == 0 {
		s.logger.Info("SKU详细信息为空，跳过映射数据填充")
		return nil
	}

	// 构建增强的映射数据，为每个SKU查询映射关系
	enrichedMappingDataList, err := s.buildEnrichedMappingData(temuProduct, skuDetails, storeID)
	if err != nil {
		return fmt.Errorf("构建增强映射数据失败: %w", err)
	}

	// 将增强的映射数据序列化为JSON并存储到Attributes字段
	attributesJSON, err := json.Marshal(enrichedMappingDataList)
	if err != nil {
		return fmt.Errorf("序列化映射数据失败: %w", err)
	}

	productData.Attributes = string(attributesJSON)

	// 使用第一个找到的映射信息填充产品级别的ProductID和Region
	s.fillProductLevelMappingInfo(productData, enrichedMappingDataList)

	return nil
}

// serializeMappingData 序列化映射数据
func (s *productSyncServiceImpl) serializeMappingData(mappingDataList []*models.TemuMappingData) ([]byte, error) {
	return json.Marshal(mappingDataList)
}
