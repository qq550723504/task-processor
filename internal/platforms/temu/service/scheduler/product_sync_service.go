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
	mappingClient    managementapi.ProductImportMappingAPI
	storeAPI         managementapi.StoreAPI
	config           *ProductSyncConfig
	logger           *logrus.Entry
}

// NewProductSyncService 创建TEMU产品同步服务
func NewProductSyncService(
	managementClient *management.ClientManager,
	productAPI *services.ProductAPI,
	mappingClient managementapi.ProductImportMappingAPI,
	storeAPI managementapi.StoreAPI,
	config *ProductSyncConfig,
) ProductSyncService {
	if config == nil {
		config = &ProductSyncConfig{
			PageSize:        100,
			MaxPages:        0, // 0表示不限制
			Language:        "en",
			IncludeInactive: false,
		}
	}

	return &productSyncServiceImpl{
		managementClient: managementClient,
		productAPI:       productAPI,
		mappingClient:    mappingClient,
		storeAPI:         storeAPI,
		config:           config,
		logger:           logrus.WithField("component", "TemuProductSyncService"),
	}
}

// FetchProductList 获取TEMU产品列表
func (s *productSyncServiceImpl) FetchProductList(ctx context.Context) ([]models.TemuProductResponse, error) {
	s.logger.Debug("开始获取TEMU产品列表")

	var allProducts []models.TemuProductResponse
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
			s.logger.Infof("达到最大页数限制: %d", s.config.MaxPages)
			break
		}

		// 调用TEMU API获取产品列表
		var products []models.TemuProductResponse
		var err error

		if s.config.IncludeInactive {
			// 获取所有产品（包括未上架的）
			response, apiErr := s.productAPI.ListProducts(pageNo, s.config.PageSize)
			if apiErr != nil {
				return nil, fmt.Errorf("获取TEMU产品列表失败(页面%d): %w", pageNo, apiErr)
			}
			products = response.Result.GoodsList
		} else {
			// 只获取已上架的产品
			products, err = s.productAPI.ListOnShelfProducts(pageNo, s.config.PageSize)
			if err != nil {
				return nil, fmt.Errorf("获取TEMU已上架产品列表失败(页面%d): %w", pageNo, err)
			}
		}

		s.logger.Debugf("页面%d获取到%d个产品", pageNo, len(products))
		allProducts = append(allProducts, products...)

		// 如果返回的产品数量小于页面大小，说明已经是最后一页
		if len(products) < s.config.PageSize {
			break
		}
		pageNo++
	}

	s.logger.Infof("获取TEMU产品列表完成，共%d个产品", len(allProducts))
	return allProducts, nil
}

// ConvertProducts 转换TEMU产品格式为管理系统格式
func (s *productSyncServiceImpl) ConvertProducts(ctx context.Context, products []models.TemuProductResponse, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	totalCount := len(products)
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
		"count":     totalCount,
	}).Info("开始转换TEMU产品格式")

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

		productData, err := s.convertSingleProduct(ctx, &temuProduct, tenantID, storeID)
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

	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": len(productDataList),
		"failed":  totalCount - len(productDataList),
	}).Info("转换TEMU产品格式完成")

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

	successCount := 0
	failedCount := 0
	firstProduct := productDataList[0]
	productDataAPI := s.managementClient.GetProductDataClient(firstProduct.StoreID)

	// 计算进度输出间隔
	progressInterval := s.calculateProgressInterval(totalCount)

	for i, productData := range productDataList {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return successCount, ctx.Err()
		default:
		}

		if err := productDataAPI.BatchCreateOrUpdate([]*managementapi.ProductDataDTO{productData}); err != nil {
			s.logger.WithFields(logrus.Fields{
				"goods_id": productData.PlatformProductID,
				"error":    err,
			}).Error("保存TEMU产品失败")
			failedCount++
			continue
		}
		successCount++

		// 输出进度日志
		s.logProgress(i+1, totalCount, successCount, progressInterval, "产品保存进度")
	}

	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": successCount,
		"failed":  failedCount,
	}).Info("保存TEMU产品完成")

	return successCount, nil
}
