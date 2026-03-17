// Package product 提供产品领域仓储接口定义
package product

import (
	"context"
	"task-processor/internal/model"
)

// CacheRepository 缓存仓储接口
type CacheRepository interface {
	// GetFromCache 从缓存获取产品数据
	GetFromCache(ctx context.Context, req *FetchRequest) (*model.Product, error)

	// SaveToCache 保存产品数据到缓存
	SaveToCache(ctx context.Context, req *FetchRequest, product *model.Product) error

	// SaveVariantsBatch 批量保存变体数据到缓存
	SaveVariantsBatch(ctx context.Context, req *FetchRequest, variants []*model.Product) error

	// DeleteFromCache 从缓存删除产品数据
	DeleteFromCache(ctx context.Context, req *FetchRequest) error

	// ExistsInCache 检查缓存中是否存在产品数据
	ExistsInCache(ctx context.Context, req *FetchRequest) (bool, error)
}

// CrawlerRepository 爬虫仓储接口
type CrawlerRepository interface {
	// ShouldUseCrawler 判断是否应该使用爬虫
	ShouldUseCrawler(platform string) bool

	// FetchFromCrawler 使用爬虫获取产品数据
	FetchFromCrawler(ctx context.Context, req *FetchRequest) (*model.Product, error)

	// FetchVariantsBatch 批量获取变体数据
	FetchVariantsBatch(ctx context.Context, req *FetchRequest, productIDs []string) ([]*model.Product, []error)

	// GetSupportedPlatforms 获取支持的平台列表
	GetSupportedPlatforms() []string
}
