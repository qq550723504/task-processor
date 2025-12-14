package temu

import (
	"fmt"
	"time"

	"task-processor/common/amazon"
	"task-processor/common/amazon/model"
	"task-processor/common/management/api"
	"task-processor/common/product"
	"task-processor/platforms/common"

	"github.com/sirupsen/logrus"
)

// TemuProductMonitorService TEMU 产品监控服务（价格+库存）
type TemuProductMonitorService struct {
	*common.BaseMonitorService
	syncService     *SyncService
	amazonProcessor *amazon.AmazonProcessor
}

// NewTemuProductMonitorService 创建 TEMU 产品监控服务
func NewTemuProductMonitorService(
	config *common.MonitorConfig,
	mappingClient api.ProductImportMappingAPI,
	eventHandler common.MonitorEventHandler,
	syncService *SyncService,
	amazonProcessor *amazon.AmazonProcessor,
) *TemuProductMonitorService {
	return &TemuProductMonitorService{
		BaseMonitorService: common.NewBaseMonitorService(config, mappingClient, eventHandler),
		syncService:        syncService,
		amazonProcessor:    amazonProcessor,
	}
}

// GetPlatformName 获取平台名称
func (s *TemuProductMonitorService) GetPlatformName() string {
	return "TEMU"
}

// Start 启动产品监控服务
func (s *TemuProductMonitorService) Start() error {
	logrus.Info("启动 TEMU 产品监控服务（价格+库存）")

	ticker := time.NewTicker(s.Config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logrus.Debug("执行 TEMU 产品检查任务")
			// TODO: 遍历所有注册的店铺进行检查
			// 需要实现店铺注册机制
		case <-s.StopChan:
			logrus.Info("TEMU 产品监控服务已停止")
			return nil
		}
	}
}

// CheckProductChanges 检查产品变化（价格+库存）
func (s *TemuProductMonitorService) CheckProductChanges(storeID, tenantID int64) error {
	logger := logrus.WithFields(logrus.Fields{
		"platform":  "TEMU",
		"store_id":  storeID,
		"tenant_id": tenantID,
	})

	logger.Info("开始检查 TEMU 产品变化（价格+库存）")

	products, err := s.getStoreMappings(storeID, tenantID)
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

	for _, prod := range products {
		if prod.ProductID == "" {
			continue
		}

		asin := prod.ProductID
		region := prod.Region
		if region == "" {
			region = "US"
		}

		zipcode := product.GetZipcodeForRegion(region, nil)

		// 从 Amazon 获取产品信息（一次获取价格和库存）
		amazonProduct, err := fetchAmazonProduct(s.amazonProcessor, asin, region, zipcode)
		if err != nil {
			logger.WithError(err).WithField("asin", asin).Warn("获取 Amazon 产品信息失败")
			continue
		}

		// 检查价格变化
		if s.checkAndNotifyPriceChange(prod, amazonProduct, tenantID, storeID) {
			priceChangeCount++
		}

		// 检查库存变化
		if s.checkAndNotifyStockChange(prod, amazonProduct, tenantID, storeID) {
			stockChangeCount++
		}
	}

	logger.Infof("产品检查完成，价格变化: %d, 库存变化: %d", priceChangeCount, stockChangeCount)
	return nil
}

// getStoreMappings 获取店铺的所有产品数据
func (s *TemuProductMonitorService) getStoreMappings(storeID, tenantID int64) ([]*api.ProductDataDTO, error) {
	shelfStatus := api.ShelfStatusOnShelf
	productDataClient := s.syncService.repositoryFactory(storeID, tenantID)
	products, err := productDataClient.ListByStore("TEMU", tenantID, storeID, &shelfStatus)
	if err != nil {
		return nil, fmt.Errorf("查询店铺产品列表失败: %w", err)
	}
	return products, nil
}

// checkAndNotifyPriceChange 检查并通知价格变化
func (s *TemuProductMonitorService) checkAndNotifyPriceChange(
	prod *api.ProductDataDTO,
	amazonProduct *model.Product,
	tenantID, storeID int64,
) bool {
	oldPrice := parsePrice(prod.OriginalPrice.String())
	if oldPrice <= 0 {
		oldPrice = parsePrice(prod.SpecialPrice.String())
	}

	newPrice := amazonProduct.FinalPrice

	if oldPrice > 0 && newPrice > 0 {
		changePercent := ((newPrice - oldPrice) / oldPrice) * 100

		if abs(changePercent) >= s.Config.PriceChangeThreshold {
			event := &common.PriceChangeEvent{
				TenantID:          tenantID,
				StoreID:           storeID,
				Platform:          "TEMU",
				ProductID:         prod.ProductID,
				SKU:               prod.ProductID,
				OldPrice:          oldPrice,
				NewPrice:          newPrice,
				ChangePercent:     changePercent,
				PlatformProductID: prod.PlatformProductID,
				Timestamp:         time.Now(),
			}

			if err := s.EventHandler.OnPriceChange(event); err != nil {
				logrus.WithError(err).Error("处理价格变化事件失败")
				return false
			}
			return true
		}
	}

	return false
}

// checkAndNotifyStockChange 检查并通知库存变化
func (s *TemuProductMonitorService) checkAndNotifyStockChange(
	prod *api.ProductDataDTO,
	amazonProduct *model.Product,
	tenantID, storeID int64,
) bool {
	oldStock := parseStock(prod.Stock.String())
	newStock := extractStockFromProduct(amazonProduct)

	changeAmount := newStock - oldStock

	if absInt(changeAmount) >= s.Config.StockChangeThreshold {
		event := &common.StockChangeEvent{
			TenantID:          tenantID,
			StoreID:           storeID,
			Platform:          "TEMU",
			ProductID:         prod.ProductID,
			SKU:               prod.ProductID,
			OldStock:          oldStock,
			NewStock:          newStock,
			ChangeAmount:      changeAmount,
			PlatformProductID: prod.PlatformProductID,
			Timestamp:         time.Now(),
		}

		if err := s.EventHandler.OnStockChange(event); err != nil {
			logrus.WithError(err).Error("处理库存变化事件失败")
			return false
		}
		return true
	}

	return false
}
