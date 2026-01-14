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

// InventorySyncService 库存同步服务接口
type InventorySyncService interface {
	// FetchProductsForInventorySync 获取需要同步库存的产品列表
	FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)

	// FetchInventoryFromShein 从SHEIN API获取库存信息
	FetchInventoryFromShein(ctx context.Context, products []*managementapi.ProductDataDTO) (map[string]*product.InventoryQueryResponse, error)

	// UpdateInventoryToManagement 更新库存到管理系统
	UpdateInventoryToManagement(ctx context.Context, products []*managementapi.ProductDataDTO, inventoryMap map[string]*product.InventoryQueryResponse) (int, error)
}

// inventorySyncServiceImpl 库存同步服务实现
type inventorySyncServiceImpl struct {
	managementClient *management.ClientManager
	productAPI       repo.ProductAPIInterface
	logger           *logrus.Entry
}

// NewInventorySyncService 创建库存同步服务
func NewInventorySyncService(
	managementClient *management.ClientManager,
	productAPI repo.ProductAPIInterface,
) InventorySyncService {
	return &inventorySyncServiceImpl{
		managementClient: managementClient,
		productAPI:       productAPI,
		logger:           logrus.WithField("component", "InventorySyncService"),
	}
}

// FetchProductsForInventorySync 获取需要同步库存的产品列表
func (s *inventorySyncServiceImpl) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Debug("开始获取需要同步库存的产品")

	// 从管理系统获取已上架的产品列表
	productDataAPI := s.managementClient.GetProductDataClient(storeID)

	// 只获取已上架的产品 (ShelfStatus = 2)
	shelfStatus := managementapi.ShelfStatusOnShelf
	products, err := productDataAPI.ListByStore("SHEIN", tenantID, storeID, &shelfStatus)
	if err != nil {
		s.logger.Errorf("从管理系统获取产品列表失败: %v", err)
		return nil, fmt.Errorf("获取产品列表失败: %w", err)
	}

	s.logger.Infof("获取到 %d 个需要同步库存的产品", len(products))
	return products, nil
}

// FetchInventoryFromShein 从SHEIN API获取库存信息
func (s *inventorySyncServiceImpl) FetchInventoryFromShein(ctx context.Context, products []*managementapi.ProductDataDTO) (map[string]*product.InventoryQueryResponse, error) {
	s.logger.WithField("count", len(products)).Debug("开始从SHEIN获取库存信息")

	inventoryMap := make(map[string]*product.InventoryQueryResponse)

	for _, prod := range products {
		// 使用 PlatformProductID (SpuName) 查询库存
		if prod.PlatformProductID == "" {
			s.logger.Warnf("产品 [%s] 缺少 PlatformProductID，跳过", prod.Title)
			continue
		}

		// 调用 SHEIN API 查询库存详情
		inventoryResp, err := s.productAPI.QueryInventory(prod.PlatformProductID)
		if err != nil {
			s.logger.Errorf("查询产品 [%s] 库存失败: %v", prod.PlatformProductID, err)
			continue
		}

		inventoryMap[prod.PlatformProductID] = inventoryResp
		s.logger.Debugf("成功获取产品 [%s] 的库存信息", prod.PlatformProductID)
	}

	s.logger.Infof("从SHEIN获取库存信息完成，成功获取 %d 个产品的库存", len(inventoryMap))
	return inventoryMap, nil
}

// UpdateInventoryToManagement 更新库存到管理系统
func (s *inventorySyncServiceImpl) UpdateInventoryToManagement(ctx context.Context, products []*managementapi.ProductDataDTO, inventoryMap map[string]*product.InventoryQueryResponse) (int, error) {
	s.logger.WithField("count", len(products)).Debug("开始更新库存到管理系统")

	if len(inventoryMap) == 0 {
		s.logger.Info("没有库存数据需要更新")
		return 0, nil
	}

	successCount := 0
	productDataAPI := s.managementClient.GetProductDataClient(products[0].StoreID)

	for _, prod := range products {
		// 获取对应的库存信息
		inventoryResp, exists := inventoryMap[prod.PlatformProductID]
		if !exists {
			continue
		}

		// 计算总库存
		totalStock := s.calculateTotalStock(inventoryResp)

		// 更新产品的库存字段
		prod.Stock = managementapi.FlexibleString(fmt.Sprintf("%d", totalStock))

		// 保存到管理系统
		if err := productDataAPI.CreateOrUpdate(prod); err != nil {
			s.logger.Errorf("更新产品 [%s] 库存失败: %v", prod.PlatformProductID, err)
			continue
		}

		successCount++
		s.logger.Debugf("成功更新产品 [%s] 库存: %d", prod.PlatformProductID, totalStock)
	}

	s.logger.Infof("更新库存到管理系统完成: 总数=%d, 成功=%d, 失败=%d",
		len(products), successCount, len(products)-successCount)
	return successCount, nil
}

// calculateTotalStock 计算总库存
func (s *inventorySyncServiceImpl) calculateTotalStock(inventoryResp *product.InventoryQueryResponse) int {
	totalStock := 0

	// 遍历所有 SKC
	for _, skcInfo := range inventoryResp.Info.SkcInfo {
		// 遍历所有 SKU
		for _, skuInfo := range skcInfo.SkuInfo {
			// 遍历所有仓库
			for _, warehouse := range skuInfo.InventoryInfo {
				// 累加可用库存
				totalStock += warehouse.UsableInventory
			}
		}
	}

	return totalStock
}
