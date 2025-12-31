// Package shein 提供SHEIN产品监控服务核心功能
package shein

import (
	"fmt"
	"time"

	"task-processor/internal/common"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// SheinProductMonitorService SHEIN 产品监控服务（价格+库存）
type SheinProductMonitorService struct {
	*common.BaseMonitorService
	syncService             *SyncService
	apiClients              map[int64]*ShopAPIClient
	productDataManager      *ProductDataManager
	changeDetector          *ChangeDetector
	strategyManager         *StrategyManager
	operationStrategyClient api.OperationStrategyAPI
	storeClient             api.StoreAPI
}

// NewSheinProductMonitorService 创建 SHEIN 产品监控服务
func NewSheinProductMonitorService(
	config *common.MonitorConfig,
	mappingClient api.ProductImportMappingAPI,
	eventHandler common.MonitorEventHandler,
	syncService *SyncService,
	amazonProcessor *amazon.AmazonProcessor,
	rawJsonDataClient api.RawJsonDataAPI,
	inventoryRecordClient api.InventoryRecordAPI,
	operationStrategyClient api.OperationStrategyAPI,
	storeClient api.StoreAPI,
) *SheinProductMonitorService {
	// 创建产品数据管理器
	productDataManager := NewProductDataManager(
		amazonProcessor,
		rawJsonDataClient,
		inventoryRecordClient,
		syncService,
	)

	// 创建变化检测器
	changeDetector := NewChangeDetector(config, eventHandler)

	// 创建策略管理器
	strategyManager := NewStrategyManager(operationStrategyClient)

	return &SheinProductMonitorService{
		BaseMonitorService:      common.NewBaseMonitorService(config, mappingClient, eventHandler),
		syncService:             syncService,
		apiClients:              make(map[int64]*ShopAPIClient),
		productDataManager:      productDataManager,
		changeDetector:          changeDetector,
		strategyManager:         strategyManager,
		operationStrategyClient: operationStrategyClient,
		storeClient:             storeClient,
	}
}

// RegisterStore 注册店铺 API 客户端
func (s *SheinProductMonitorService) RegisterStore(storeID int64, apiClient *ShopAPIClient) {
	s.apiClients[storeID] = apiClient
	logrus.WithField("store_id", storeID).Info("注册 SHEIN 店铺到产品监控服务")
}

// GetPlatformName 获取平台名称
func (s *SheinProductMonitorService) GetPlatformName() string {
	return "SHEIN"
}

// Start 启动产品监控服务
func (s *SheinProductMonitorService) Start() error {
	logrus.Info("启动 SHEIN 产品监控服务（价格+库存）")

	ticker := time.NewTicker(s.Config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for storeID, apiClient := range s.apiClients {
				tenantID := apiClient.GetTenantID()
				if err := s.CheckProductChanges(storeID, tenantID); err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"store_id":  storeID,
						"tenant_id": tenantID,
					}).Error("SHEIN 产品检查失败")
				}
			}
		case <-s.StopChan:
			logrus.Info("SHEIN 产品监控服务已停止")
			return nil
		}
	}
}

// CheckProductChanges 检查产品变化（价格+库存）
func (s *SheinProductMonitorService) CheckProductChanges(storeID, tenantID int64) error {
	logger := logrus.WithFields(logrus.Fields{
		"platform":  "SHEIN",
		"store_id":  storeID,
		"tenant_id": tenantID,
	})

	logger.Info("开始检查 SHEIN 产品变化（价格+库存）")

	// 获取店铺信息
	storeInfo, err := s.storeClient.GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 获取店铺的价格类型配置(默认使用特价)
	priceType := "special"
	if storeInfo.PriceType != "" {
		priceType = storeInfo.PriceType
	}
	logger.WithField("price_type", priceType).Info("使用店铺配置的价格类型")

	// 获取店铺的运营策略
	strategy, err := s.operationStrategyClient.GetOperationStrategyByStoreId(storeID)
	if err != nil {
		logger.WithError(err).Warn("获取运营策略失败，将跳过策略执行")
	} else if strategy != nil && strategy.IsEnabled() {
		logger.WithField("strategy_name", strategy.Name).Info("已加载运营策略")
	}

	// 获取店铺产品数据
	products, err := s.productDataManager.GetStoreMappings(storeID, tenantID)
	if err != nil {
		return fmt.Errorf("查询店铺产品失败: %w", err)
	}

	if len(products) == 0 {
		logger.Info("该店铺没有已上架产品")
		return nil
	}

	logger.Infof("找到 %d 个已上架产品", len(products))

	priceChangeCount := 0
	stockChangeCount := 0
	strategyExecutedCount := 0

	for _, prod := range products {
		// 从 Attributes 中解析所有 SKU 映射数据（包含映射信息和库存）
		skuMappingList := extractMappingInfoFromAttributes(prod.Attributes)
		if len(skuMappingList) == 0 {
			logger.WithField("product_id", prod.ProductID).Debug("产品没有映射信息，跳过")
			continue
		}

		// 遍历所有 SKU 的映射数据
		for _, skuMapping := range skuMappingList {
			mappingInfo := skuMapping.MappingInfo
			asin := mappingInfo.ProductID
			if asin == "" {
				logger.WithField("platform_sku", mappingInfo.SKU).Debug("映射信息中没有 ASIN，跳过")
				continue
			}

			// 获取Amazon产品数据
			amazonProduct, err := s.productDataManager.GetAmazonProductData(asin, mappingInfo, tenantID, storeID, priceType)
			if err != nil {
				logger.WithError(err).WithFields(logrus.Fields{
					"asin":             asin,
					"platform_sku":     mappingInfo.SKU,
					"platform_product": prod.ProductID,
				}).Warn("获取 Amazon 产品信息失败")
				continue
			}

			// 无论是否变化，都更新 attributes 中的 Amazon 数据
			go s.productDataManager.UpdateAttributesWithAmazonData(prod, mappingInfo.SKU, amazonProduct, tenantID, storeID, priceType)

			// 检查价格变化
			if s.changeDetector.CheckAndNotifyPriceChange(prod, amazonProduct, skuMapping, tenantID, storeID, priceType) {
				priceChangeCount++
			}

			// 检查库存变化
			if s.changeDetector.CheckAndNotifyStockChange(prod, amazonProduct, skuMapping, tenantID, storeID) {
				stockChangeCount++
			}

			// 执行运营策略
			if strategy != nil && strategy.IsEnabled() {
				apiClient := s.apiClients[storeID]
				if apiClient != nil {
					executedCount := s.strategyManager.ExecuteStrategy(strategy, apiClient, prod, skuMapping, amazonProduct, logger)
					strategyExecutedCount += executedCount
				}
			}
		}
	}

	logger.Infof("产品检查完成，价格变化: %d, 库存变化: %d, 策略执行: %d", priceChangeCount, stockChangeCount, strategyExecutedCount)
	return nil
}
