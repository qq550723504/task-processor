// Package product 提供产品数据获取业务逻辑
package product

import (
	"context"
	"fmt"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// ProductService 产品数据业务服务
type ProductService struct {
	logger      *logrus.Entry
	cacheRepo   CacheRepository
	crawlerRepo CrawlerRepository
	validator   *ProductValidator
}

// NewProductService 创建产品数据业务服务
func NewProductService(
	cacheRepo CacheRepository,
	crawlerRepo CrawlerRepository,
	validator *ProductValidator,
	logger *logrus.Entry,
) *ProductService {
	return &ProductService{
		logger:      logger.WithField("component", "ProductService"),
		cacheRepo:   cacheRepo,
		crawlerRepo: crawlerRepo,
		validator:   validator,
	}
}

// FetchProduct 获取产品数据（优先从缓存，如果没有则从爬虫）
func (s *ProductService) FetchProduct(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	if err := s.validator.ValidateRequest(req); err != nil {
		return nil, fmt.Errorf("请求参数验证失败: %w", err)
	}

	s.logger.Infof("开始获取产品数据: ProductID=%s, Platform=%s, Region=%s",
		req.ProductID, req.Platform, req.Region)

	// 第一步：尝试从缓存获取
	product, err := s.fetchFromCache(ctx, req)
	if err == nil && product != nil {
		return product, nil
	}

	if err != nil {
		s.logger.Debugf("缓存获取失败: %v", err)
	}

	// 第二步：使用爬虫获取
	product, err = s.fetchFromCrawler(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("爬虫获取失败: %w", err)
	}

	// 第三步：异步保存到缓存
	go s.saveToCacheAsync(context.Background(), req, product)

	return product, nil
}

// CacheProduct 缓存产品数据
func (s *ProductService) CacheProduct(ctx context.Context, req *FetchRequest, product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据为空")
	}

	s.logger.Infof("开始缓存产品数据: ProductID=%s", req.ProductID)
	return s.cacheRepo.SaveToCache(ctx, req, product)
}

// CacheVariants 批量缓存变体数据
func (s *ProductService) CacheVariants(ctx context.Context, req *FetchRequest, variants []*model.Product) error {
	if len(variants) == 0 {
		s.logger.Debug("没有变体数据，跳过缓存")
		return nil
	}

	s.logger.Infof("开始批量缓存变体数据: 数量=%d", len(variants))
	return s.cacheRepo.SaveVariantsBatch(ctx, req, variants)
}

// fetchFromCache 从缓存获取产品数据
func (s *ProductService) fetchFromCache(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	product, err := s.cacheRepo.GetFromCache(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.validator.ValidateProduct(product); err != nil {
		s.logger.Warnf("缓存数据验证失败: %v", err)
		return nil, err
	}

	return product, nil
}

// fetchFromCrawler 从爬虫获取产品数据
func (s *ProductService) fetchFromCrawler(ctx context.Context, req *FetchRequest) (*model.Product, error) {
	if !s.crawlerRepo.ShouldUseCrawler(req.Platform) {
		return nil, fmt.Errorf("平台 %s 不支持爬虫获取", req.Platform)
	}

	s.logger.Infof("使用爬虫抓取: ProductID=%s", req.ProductID)

	product, err := s.crawlerRepo.FetchFromCrawler(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.validator.ValidateProduct(product); err != nil {
		return nil, fmt.Errorf("爬取数据验证失败: %w", err)
	}

	return product, nil
}

// saveToCacheAsync 异步保存到缓存
func (s *ProductService) saveToCacheAsync(ctx context.Context, req *FetchRequest, product *model.Product) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("异步保存缓存时发生panic: %v", r)
		}
	}()

	if err := s.cacheRepo.SaveToCache(ctx, req, product); err != nil {
		s.logger.Warnf("保存到缓存失败: %v", err)
	} else {
		s.logger.Debugf("产品 %s 已保存到缓存", req.ProductID)
	}
}
