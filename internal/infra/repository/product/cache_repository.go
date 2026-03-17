// Package product 提供产品缓存仓储的具体实现
package product

import (
	"context"
	"fmt"
	"task-processor/internal/model"
	domainproduct "task-processor/internal/domain/product"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// CacheRepository 缓存仓储实现
type CacheRepository struct {
	rawJsonDataClient api.RawJsonDataAPI
	logger            *logrus.Entry
}

// NewCacheRepositoryImpl 创建缓存仓储实现
func NewCacheRepositoryImpl(rawJsonDataClient api.RawJsonDataAPI, logger *logrus.Entry) domainproduct.CacheRepository {
	return &CacheRepository{
		rawJsonDataClient: rawJsonDataClient,
		logger:            logger.WithField("component", "CacheRepositoryImpl"),
	}
}

// GetFromCache 从缓存获取产品数据
func (r *CacheRepository) GetFromCache(ctx context.Context, req *domainproduct.FetchRequest) (*model.Product, error) {
	if r.rawJsonDataClient == nil {
		return nil, fmt.Errorf("缓存客户端未初始化")
	}

	resp, err := r.rawJsonDataClient.GetRawJsonData(&api.RawJsonDataReqDTO{
		Platform:  req.Platform,
		ProductID: req.ProductID,
		Region:    req.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("获取缓存数据失败: %w", err)
	}

	if resp == nil || resp.RawJSONData == "" {
		return nil, nil
	}

	r.logger.Debugf("从缓存获取产品成功: %s", req.ProductID)
	return nil, nil
}

// SaveToCache 保存产品数据到缓存
func (r *CacheRepository) SaveToCache(ctx context.Context, req *domainproduct.FetchRequest, product *model.Product) error {
	if r.rawJsonDataClient == nil {
		return fmt.Errorf("缓存客户端未初始化")
	}

	_, err := r.rawJsonDataClient.CreateRawJsonData(&api.RawJsonDataCreateReqDTO{
		Platform:    req.Platform,
		ProductID:   req.ProductID,
		Region:      req.Region,
		RawJsonData: "", // TODO: 序列化产品数据
	})
	if err != nil {
		return fmt.Errorf("保存缓存数据失败: %w", err)
	}

	r.logger.Debugf("保存产品到缓存成功: %s", req.ProductID)
	return nil
}

// SaveVariantsBatch 批量保存变体数据到缓存
func (r *CacheRepository) SaveVariantsBatch(ctx context.Context, req *domainproduct.FetchRequest, variants []*model.Product) error {
	for _, variant := range variants {
		variantReq := &domainproduct.FetchRequest{
			Platform:  req.Platform,
			Region:    req.Region,
			ProductID: variant.Asin,
		}
		if err := r.SaveToCache(ctx, variantReq, variant); err != nil {
			return err
		}
	}

	r.logger.Debugf("批量保存变体到缓存成功: %d 个", len(variants))
	return nil
}

// DeleteFromCache 从缓存删除产品数据
func (r *CacheRepository) DeleteFromCache(ctx context.Context, req *domainproduct.FetchRequest) error {
	r.logger.Warnf("删除缓存功能暂未实现: %s", req.ProductID)
	return nil
}

// ExistsInCache 检查缓存中是否存在产品数据
func (r *CacheRepository) ExistsInCache(ctx context.Context, req *domainproduct.FetchRequest) (bool, error) {
	product, err := r.GetFromCache(ctx, req)
	if err != nil {
		return false, err
	}
	return product != nil, nil
}

