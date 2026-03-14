// Package repository 提供缓存仓储的具体实现
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product/repo"
	"task-processor/internal/domain/product/types"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// CacheRepositoryImpl 缓存仓储实现
type CacheRepositoryImpl struct {
	rawJsonDataClient management.RawJsonDataAPIClient
	logger            *logrus.Entry
}

// NewCacheRepositoryImpl 创建缓存仓储实现
func NewCacheRepositoryImpl(rawJsonDataClient management.RawJsonDataAPIClient, logger *logrus.Entry) repo.CacheRepository {
	return &CacheRepositoryImpl{
		rawJsonDataClient: rawJsonDataClient,
		logger:            logger.WithField("component", "CacheRepositoryImpl"),
	}
}

// GetFromCache 从缓存获取产品数据
func (r *CacheRepositoryImpl) GetFromCache(ctx context.Context, req *types.FetchRequest) (*model.Product, error) {
	// 构建查询请求
	queryReq := &api.RawJsonDataReqDTO{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		ProductID:  req.ProductID,
		Region:     req.Region,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	}

	// 查询原始数据
	rawData, err := r.rawJsonDataClient.GetRawJsonData(queryReq)
	if err != nil {
		return nil, fmt.Errorf("查询缓存数据失败: %w", err)
	}

	if rawData == nil || rawData.RawJSONData == "" {
		return nil, fmt.Errorf("缓存中没有找到产品数据")
	}

	// 解析JSON数据
	var product model.Product
	if err := json.Unmarshal([]byte(rawData.RawJSONData), &product); err != nil {
		return nil, fmt.Errorf("解析缓存数据失败: %w", err)
	}

	r.logger.Debugf("从缓存获取产品成功: ProductID=%s", req.ProductID)
	return &product, nil
}

// SaveToCache 保存产品数据到缓存
func (r *CacheRepositoryImpl) SaveToCache(ctx context.Context, req *types.FetchRequest, product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据不能为空")
	}

	// 序列化产品数据
	jsonData, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("序列化产品数据失败: %w", err)
	}

	// 构建保存请求
	saveReq := &api.RawJsonDataCreateReqDTO{
		Platform:    req.Platform,
		Region:      req.Region,
		ProductID:   req.ProductID,
		RawJsonData: string(jsonData),
		Creator:     req.Creator,
	}

	// 保存到缓存
	id, err := r.rawJsonDataClient.CreateRawJsonData(saveReq)
	if err != nil {
		return fmt.Errorf("保存到缓存失败: %w", err)
	}

	r.logger.Debugf("产品保存到缓存成功: ProductID=%s, CacheID=%d", req.ProductID, id)
	return nil
}

// SaveVariantsBatch 批量保存变体数据到缓存
func (r *CacheRepositoryImpl) SaveVariantsBatch(ctx context.Context, req *types.FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		return nil
	}

	successCount := 0
	var lastError error

	for _, variant := range variants {
		if variant == nil {
			continue
		}

		// 为每个变体创建单独的请求
		variantReq := &types.FetchRequest{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			Region:     req.Region,
			ProductID:  variant.Asin, // 使用变体的ASIN
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		}

		if err := r.SaveToCache(ctx, variantReq, variant); err != nil {
			r.logger.Warnf("保存变体 %s 到缓存失败: %v", variant.Asin, err)
			lastError = err
		} else {
			successCount++
		}
	}

	r.logger.Infof("批量保存变体完成: 成功 %d/%d", successCount, len(variants))

	// 如果全部失败，返回最后一个错误
	if successCount == 0 && lastError != nil {
		return fmt.Errorf("批量保存变体全部失败: %w", lastError)
	}

	return nil
}

// DeleteFromCache 从缓存删除产品数据
func (r *CacheRepositoryImpl) DeleteFromCache(ctx context.Context, req *types.FetchRequest) error {
	// 注意：这里需要根据实际的API接口实现删除逻辑
	// 当前的 RawJsonDataAPI 接口可能没有删除方法
	r.logger.Warnf("删除缓存功能暂未实现: ProductID=%s", req.ProductID)
	return fmt.Errorf("删除缓存功能暂未实现")
}

// ExistsInCache 检查缓存中是否存在产品数据
func (r *CacheRepositoryImpl) ExistsInCache(ctx context.Context, req *types.FetchRequest) (bool, error) {
	_, err := r.GetFromCache(ctx, req)
	if err != nil {
		// 如果是找不到数据的错误，返回false
		if err.Error() == "缓存中没有找到产品数据" {
			return false, nil
		}
		// 其他错误返回错误信息
		return false, err
	}
	return true, nil
}
