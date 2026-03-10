// Package scheduler 提供TEMU产品同步服务实现
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/api/services"

	"github.com/sirupsen/logrus"
)

// productSyncServiceImpl TEMU产品同步服务实现
type productSyncServiceImpl struct {
	managementClient *management.ClientManager
	productAPI       *services.ProductAPI
	skuQueryAPI      *services.SkuQueryAPI
	mappingClient    managementapi.ProductImportMappingAPI
	storeAPI         managementapi.StoreAPI
	config           *ProductSyncConfig
	logger           *logrus.Entry
}

// NewProductSyncService 创建TEMU产品同步服务
func NewProductSyncService(
	managementClient *management.ClientManager,
	productAPI *services.ProductAPI,
	skuQueryAPI *services.SkuQueryAPI,
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
		managementClient: managementClient,
		productAPI:       productAPI,
		skuQueryAPI:      skuQueryAPI,
		mappingClient:    mappingClient,
		storeAPI:         storeAPI,
		config:           config,
		logger:           logrus.WithField("component", "TemuProductSyncService"),
	}
}

// FetchProductList 获取TEMU产品列表
func (s *productSyncServiceImpl) FetchProductList(ctx context.Context) ([]models.GoodsSearchItem, error) {
	s.logger.WithFields(logrus.Fields{
		"max_pages": s.config.MaxPages,
		"page_size": s.config.PageSize,
	}).Info("开始获取TEMU产品列表（调试模式：只处理一页数据）")

	var allProducts []models.GoodsSearchItem
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
		options := services.NewGoodsSearchOptions(pageNo, s.config.PageSize)
		response, apiErr := s.productAPI.SearchGoods(options)
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
func (s *productSyncServiceImpl) ConvertProducts(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	totalCount := len(products)

	productDataList := make([]*managementapi.ProductDataDTO, 0, totalCount)

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
func (s *productSyncServiceImpl) SaveProducts(ctx context.Context, productDataList []*managementapi.ProductDataDTO) (int, error) {
	totalCount := len(productDataList)
	s.logger.WithField("count", totalCount).Info("开始保存TEMU产品")

	if totalCount == 0 {
		s.logger.Info("没有产品需要保存")
		return 0, nil
	}

	firstProduct := productDataList[0]
	productDataAPI := s.managementClient.GetProductDataClient(firstProduct.StoreID)

	// 转换为批量请求格式
	products := make([]managementapi.ProductDataItemDTO, 0, totalCount)
	for _, productData := range productDataList {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		item := managementapi.ProductDataItemDTO{
			PlatformProductID:  productData.PlatformProductID,
			ProductName:        productData.Title,
			ProductSku:         productData.ProductID,
			ProductPrice:       productData.OriginalPrice,
			ProductStock:       productData.Stock, // 默认库存，需要根据实际情况设置
			ProductCategory:    productData.Category,
			ProductImage:       productData.MainImageURL,
			ProductDescription: productData.Description,
			ShelfStatus:        &productData.ShelfStatus,
			PublishTime:        productData.PublishTime,
			ShelfTime:          productData.ShelfTime,
			Brand:              productData.Brand,
			CategoryID:         &productData.CategoryID,
			SpecialPrice:       productData.SpecialPrice,
			PriceCurrency:      productData.PriceCurrency,
			ImageUrls:          productData.ImageURLs,
			Attributes:         productData.Attributes,
			PlatformStatus:     productData.PlatformStatus,
			PlatformData:       productData.PlatformData,
			ParentProductID:    productData.ParentProductID,
			CreateTime:         productData.CreateTime,
			UpdateTime:         productData.UpdateTime,
		}
		products = append(products, item)
	}

	// 构建批量请求
	batchReq := &managementapi.ProductDataBatchSaveReqDTO{
		Platform: firstProduct.Platform,
		TenantID: firstProduct.TenantID,
		Region:   firstProduct.Region,
		StoreID:  firstProduct.StoreID,
		Products: products,
	}

	// 执行批量保存
	successCount, err := productDataAPI.BatchCreateOrUpdate(batchReq)
	if err != nil {
		s.logger.WithError(err).Error("批量保存TEMU产品失败")
		return 0, fmt.Errorf("批量保存TEMU产品失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": successCount,
	}).Info("批量保存TEMU产品完成")

	return successCount, nil
}
