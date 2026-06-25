// package sync 提供TEMU产品同步服务实现
package sync

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/listingadmin"
	temuproduct "task-processor/internal/temu/api/product"
	temuquery "task-processor/internal/temu/api/query"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

type productDataClientFactory interface {
	GetProductDataClient(storeID int64) managementapi.ProductDataAPI
}

type productSyncRuntime interface {
	productDataClientFactory
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
}

// productSyncServiceImpl TEMU产品同步服务实现
type productSyncServiceImpl struct {
	runtime         productSyncRuntime
	productAPI      *temuproduct.API
	skuQueryAPI     *temuquery.API
	mappingClient   managementapi.ProductImportMappingAPI
	storeAPI        managementapi.StoreAPI
	storeRepo       *listingadmin.GormStoreRepository
	mappingRepo     *listingadmin.GormProductImportMappingRepository
	productDataRepo listingadmin.ProductDataRepository
	config          *ProductSyncConfig
	logger          *logrus.Entry
}

// NewProductSyncService 创建TEMU产品同步服务
func NewProductSyncService(
	runtime productSyncRuntime,
	productAPI *temuproduct.API,
	skuQueryAPI *temuquery.API,
	mappingClient managementapi.ProductImportMappingAPI,
	storeAPI managementapi.StoreAPI,
	config *ProductSyncConfig,
) ProductSyncService {
	if config == nil {
		config = &ProductSyncConfig{
			PageSize:        100,
			MaxPages:        1, // 暂时只处理一页数据用于调试
			Language:        "en",
			IncludeInactive: false,
		}
	}

	return &productSyncServiceImpl{
		runtime:         runtime,
		productAPI:      productAPI,
		skuQueryAPI:     skuQueryAPI,
		mappingClient:   mappingClient,
		storeAPI:        storeAPI,
		productDataRepo: runtime.GetLocalProductDataRepository(),
		storeRepo: func() *listingadmin.GormStoreRepository {
			if runtime == nil {
				return nil
			}
			return runtime.GetLocalStoreRepository()
		}(),
		mappingRepo: func() *listingadmin.GormProductImportMappingRepository {
			if runtime == nil {
				return nil
			}
			return runtime.GetLocalProductImportMappingRepository()
		}(),
		config: config,
		logger: logger.GetGlobalLogger("TemuProductSyncService"),
	}
}

// FetchProductList 获取TEMU产品列表
func (s *productSyncServiceImpl) FetchProductList(ctx context.Context) ([]temuproduct.GoodsSearchItem, error) {
	s.logger.WithFields(logrus.Fields{
		"max_pages": s.config.MaxPages,
		"page_size": s.config.PageSize,
	}).Info("开始获取TEMU产品列表（调试模式：只处理一页数据）")

	var allProducts []temuproduct.GoodsSearchItem
	pageNo := 1

	for {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 检查是否达到最大页数限制
		if s.config.MaxPages > 0 && pageNo > s.config.MaxPages {
			s.logger.WithFields(logrus.Fields{
				"current_page": pageNo,
				"max_pages":    s.config.MaxPages,
			}).Info("达到最大页数限制，停止获取")
			break
		}

		// 调用TEMU API获取产品列表
		options := temuproduct.NewGoodsSearchOptions(pageNo, s.config.PageSize)
		response, apiErr := s.productAPI.SearchGoodsWithOptions(options)
		if apiErr != nil {
			return nil, fmt.Errorf("获取TEMU产品列表失败(页面%d): %w", pageNo, apiErr)
		}
		products := response.Result.GoodsList

		s.logger.WithFields(logrus.Fields{
			"page_no":        pageNo,
			"products_count": len(products),
			"page_size":      s.config.PageSize,
		}).Info("成功获取页面数据")

		allProducts = append(allProducts, products...)

		// 如果返回的产品数量小于页面大小，说明已经是最后一页
		if len(products) < s.config.PageSize {
			s.logger.WithFields(logrus.Fields{
				"products_count": len(products),
				"page_size":      s.config.PageSize,
			}).Info("当前页产品数量小于页面大小，已是最后一页")
			break
		}
		pageNo++
	}

	s.logger.WithFields(logrus.Fields{
		"total_products":  len(allProducts),
		"pages_processed": pageNo,
	}).Info("获取TEMU产品列表完成")

	return allProducts, nil
}

// ConvertProducts 转换TEMU产品格式为管理系统格式
func (s *productSyncServiceImpl) ConvertProducts(ctx context.Context, products []temuproduct.GoodsSearchItem, tenantID, storeID int64) ([]*TemuProductSnapshot, error) {
	totalCount := len(products)

	productDataList := make([]*TemuProductSnapshot, 0, totalCount)

	// 计算进度输出间隔
	progressInterval := s.calculateProgressInterval(totalCount)

	for i, temuProduct := range products {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		productData, err := s.convertSingleGoodsProduct(&temuProduct, tenantID, storeID)
		if err != nil {
			s.logger.WithError(err).WithField("goods_id", temuProduct.GoodsID).Warn("转换TEMU产品失败，跳过")
			continue
		}

		if productData != nil {
			productDataList = append(productDataList, productData)
		}

		// 输出进度日志
		s.logProgress(i+1, totalCount, len(productDataList), progressInterval, "产品转换进度")
	}

	return productDataList, nil
}

// SaveProducts 保存产品到管理系统
func (s *productSyncServiceImpl) SaveProducts(ctx context.Context, productDataList []*TemuProductSnapshot) (int, error) {
	totalCount := len(productDataList)
	s.logger.WithField("count", totalCount).Info("开始保存TEMU产品")

	if totalCount == 0 {
		s.logger.Info("没有产品需要保存")
		return 0, nil
	}

	if s.productDataRepo != nil {
		items := make([]listingadmin.ProductData, 0, totalCount)
		for _, productData := range productDataList {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
			}
			items = append(items, temuProductDataFromSnapshot(productData))
		}
		successCount, err := s.productDataRepo.UpsertProductDataBatch(ctx, items)
		if err != nil {
			s.logger.WithError(err).Error("通过产品数据仓储批量保存TEMU产品失败")
			return 0, fmt.Errorf("通过产品数据仓储批量保存TEMU产品失败: %w", err)
		}
		s.logger.WithFields(logrus.Fields{
			"total":   totalCount,
			"success": successCount,
			"path":    "repository",
		}).Info("批量保存TEMU产品完成")
		return successCount, nil
	}

	firstProduct := productDataList[0]
	if s.runtime == nil {
		return 0, fmt.Errorf("product sync runtime is not initialized")
	}
	productDataAPI := s.runtime.GetProductDataClient(firstProduct.StoreID)
	if productDataAPI == nil {
		return 0, fmt.Errorf("product data client is not initialized for store %d", firstProduct.StoreID)
	}

	// 转换为批量请求格式
	products := make([]managementapi.ProductDataItemDTO, 0, totalCount)
	for _, productData := range productDataList {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		products = append(products, productData.toProductDataItemDTO())
	}

	// 构建批量请求并执行保存
	batchReq := firstProduct.toBatchSaveReq(products)

	// 执行批量保存
	successCount, err := productDataAPI.BatchCreateOrUpdate(batchReq)
	if err != nil {
		s.logger.WithError(err).Error("批量保存TEMU产品失败")
		return 0, fmt.Errorf("批量保存TEMU产品失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": successCount,
		"path":    "management_client",
	}).Info("批量保存TEMU产品完成")

	return successCount, nil
}
