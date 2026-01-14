// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/repo"

	"github.com/sirupsen/logrus"
)

// ProductSyncService 产品同步服务接口
type ProductSyncService interface {
	// FetchProductList 获取产品列表
	FetchProductList(ctx context.Context) ([]product.ProductListItem, error)

	// ConvertProducts 转换产品格式
	ConvertProducts(ctx context.Context, products []product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)

	// SaveProducts 保存产品到管理系统
	SaveProducts(ctx context.Context, productDataList []*managementapi.ProductDataDTO) (int, error)
}

// productSyncServiceImpl 产品同步服务实现
type productSyncServiceImpl struct {
	managementClient *management.ClientManager
	productAPI       repo.ProductAPIInterface
	logger           *logrus.Entry
}

// NewProductSyncService 创建产品同步服务
func NewProductSyncService(
	managementClient *management.ClientManager,
	productAPI repo.ProductAPIInterface,
) ProductSyncService {
	return &productSyncServiceImpl{
		managementClient: managementClient,
		productAPI:       productAPI,
		logger:           logrus.WithField("component", "ProductSyncService"),
	}
}

// FetchProductList 获取产品列表
func (s *productSyncServiceImpl) FetchProductList(ctx context.Context) ([]product.ProductListItem, error) {
	s.logger.Debug("开始获取产品列表")

	var allProducts []product.ProductListItem

	// 分页获取所有产品
	pageNum := 1
	const pageSize = 100

	for {
		// 构建请求参数
		req := &product.ProductListRequest{
			Language:             "en",
			OnlyRecommendResell:  false,
			OnlySpmbCopyProduct:  false,
			SearchAbandonProduct: false,
			SearchIllegal:        false,
			SearchLessInventory:  false,
			ShelfType:            "", // 空表示获取所有状态的产品
			SortType:             0,
		}

		// 调用 SHEIN API 获取产品列表
		response, err := s.productAPI.ListProducts(pageNum, pageSize, req)
		if err != nil {
			s.logger.Errorf("获取产品列表失败(页面%d): %v", pageNum, err)
			return nil, fmt.Errorf("获取产品列表失败: %w", err)
		}

		s.logger.Debugf("页面%d获取到%d个产品", pageNum, len(response.Info.Data))

		allProducts = append(allProducts, response.Info.Data...)

		// 如果当前页数据少于页面大小，说明已经到最后一页
		if len(response.Info.Data) < pageSize {
			break
		}
		pageNum++
	}

	s.logger.Infof("获取产品列表完成，共%d个产品", len(allProducts))
	return allProducts, nil
}

// ConvertProducts 转换产品格式
func (s *productSyncServiceImpl) ConvertProducts(ctx context.Context, products []product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
		"count":     len(products),
	}).Debug("开始转换产品格式")

	productDataList := make([]*managementapi.ProductDataDTO, 0, len(products))

	for _, sheinProduct := range products {
		// 转换单个产品
		productData := s.convertSingleProduct(&sheinProduct, tenantID, storeID)
		if productData != nil {
			productDataList = append(productDataList, productData)
		}
	}

	s.logger.Infof("转换产品格式完成，成功转换%d个产品", len(productDataList))
	return productDataList, nil
}

// convertSingleProduct 转换单个产品
func (s *productSyncServiceImpl) convertSingleProduct(sheinProduct *product.ProductListItem, tenantID, storeID int64) *managementapi.ProductDataDTO {
	// 基础产品数据
	productData := &managementapi.ProductDataDTO{
		TenantID:          tenantID,
		StoreID:           storeID,
		Platform:          "SHEIN",
		PlatformProductID: sheinProduct.SpuCode,
		Title:             sheinProduct.ProductNameEn,
		CategoryID:        sheinProduct.CategoryID,
		Brand:             sheinProduct.BrandName,
		ShelfStatus:       s.mapShelfStatus(sheinProduct.ShelfStatus),
	}

	// 处理SKC信息
	if len(sheinProduct.SkcInfoList) > 0 {
		// 使用第一个SKC的主图作为产品主图
		firstSkc := sheinProduct.SkcInfoList[0]
		if firstSkc.MainImageThumbnailURL != "" {
			productData.MainImageURL = firstSkc.MainImageThumbnailURL
		}
	}

	return productData
}

// mapShelfStatus 映射上架状态
func (s *productSyncServiceImpl) mapShelfStatus(status string) int {
	switch status {
	case "ON_SHELF":
		return 2 // 已上架
	case "OFF_SHELF":
		return 3 // 已下架
	default:
		return 0 // 待上架
	}
}

// SaveProducts 保存产品到管理系统
func (s *productSyncServiceImpl) SaveProducts(ctx context.Context, productDataList []*managementapi.ProductDataDTO) (int, error) {
	s.logger.WithField("count", len(productDataList)).Debug("开始保存产品")

	if len(productDataList) == 0 {
		s.logger.Info("没有产品需要保存")
		return 0, nil
	}

	successCount := 0
	// 使用第一个产品的 StoreID 和 TenantID 创建 ProductDataAPI
	firstProduct := productDataList[0]
	productDataAPI := s.managementClient.GetProductDataClient(firstProduct.StoreID)

	for _, productData := range productDataList {
		// 调用管理系统API保存产品
		if err := productDataAPI.CreateOrUpdate(productData); err != nil {
			s.logger.Errorf("保存产品失败 [%s]: %v", productData.PlatformProductID, err)
			continue
		}
		successCount++
	}

	s.logger.Infof("保存产品完成: 总数=%d, 成功=%d, 失败=%d",
		len(productDataList), successCount, len(productDataList)-successCount)
	return successCount, nil
}
