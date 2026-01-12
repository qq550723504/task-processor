// Package pricing 提供TEMU平台自动核价服务
package pricing

import (
	"fmt"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// AutoPricingService 自动核价服务
type AutoPricingService struct {
	apiClient  api.APIClientInterface
	pricingAPI *api.PricingAPI
	logger     *logrus.Entry
}

// NewAutoPricingService 创建自动核价服务
func NewAutoPricingService(apiClient api.APIClientInterface) *AutoPricingService {
	logger := logrus.WithFields(logrus.Fields{
		"component": "AutoPricingService",
		"tenantID":  apiClient.GetTenantID(),
		"storeID":   apiClient.GetStoreID(),
	})

	return &AutoPricingService{
		apiClient:  apiClient,
		pricingAPI: api.NewPricingAPI(apiClient, logger),
		logger:     logger,
	}
}

// AutoProcessPendingPricesWithRules 根据利润率规则智能处理待核价商品
func (s *AutoPricingService) AutoProcessPendingPricesWithRules(managementClient *management.ClientManager) (*models.PricingStatistics, error) {
	s.logger.Info("开始智能核价处理")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	// 使用基础决策服务
	s.logger.Info("使用基础决策服务处理待核价商品")
	decisionService := NewPricingDecisionService(managementClient, s.apiClient.GetTenantID(), s.apiClient.GetStoreID())
	if decisionService == nil {
		return nil, fmt.Errorf("创建决策服务失败")
	}

	return s.processWithService(decisionService)
}

// AutoProcessPendingPricesWithRulesAndAmazon 根据利润率规则智能处理待核价商品（支持Amazon数据）
func (s *AutoPricingService) AutoProcessPendingPricesWithRulesAndAmazon(
	managementClient *management.ClientManager,
	configProvider temu.ConfigProvider,
) (*models.PricingStatistics, error) {
	s.logger.Info("开始智能核价处理（Amazon增强版）")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	if configProvider == nil {
		s.logger.Warn("配置提供者为空，使用基础决策服务")
		return s.AutoProcessPendingPricesWithRules(managementClient)
	}

	// 获取Amazon配置和处理器
	amazonConfig := configProvider.GetAmazonConfig()
	amazonProcessor := configProvider.GetAmazonProcessor()
	platformConfig := configProvider.GetPlatformConfig() // 获取平台配置

	if amazonConfig == nil || !amazonConfig.Enabled {
		s.logger.Warn("Amazon配置未启用，使用基础决策服务")
		return s.AutoProcessPendingPricesWithRules(managementClient)
	}

	if amazonProcessor == nil {
		s.logger.Warn("Amazon处理器未初始化，使用基础决策服务")
		return s.AutoProcessPendingPricesWithRules(managementClient)
	}

	// 创建支持Amazon的决策服务
	s.logger.Info("使用Amazon增强的决策服务")
	decisionService := NewPricingDecisionServiceWithAmazon(
		managementClient,
		s.apiClient.GetTenantID(),
		s.apiClient.GetStoreID(),
		amazonConfig,
		amazonProcessor,
		platformConfig, // 传递平台配置
	)

	if decisionService == nil {
		return nil, fmt.Errorf("创建Amazon增强决策服务失败")
	}

	return s.processWithService(decisionService)
}

// processWithService 使用指定的决策服务处理待核价商品
func (s *AutoPricingService) processWithService(decisionService *PricingDecisionService) (*models.PricingStatistics, error) {
	stats := &models.PricingStatistics{}
	pageNo := 1
	pageSize := 25

	for {
		// 获取待核价列表
		resp, err := s.pricingAPI.GetPendingPriceList(pageNo, pageSize)
		if err != nil {
			return stats, fmt.Errorf("获取待核价列表失败: %w", err)
		}

		if resp == nil || len(resp.Result.SalesBoostGoodsList) == 0 {
			s.logger.Info("没有更多待核价商品")
			break
		}

		// 遍历商品列表
		for _, goods := range resp.Result.SalesBoostGoodsList {
			// 遍历每个商品的SKU列表
			for _, sku := range goods.SalesBoostSkuList {
				stats.TotalProcessed++

				// 做出决策
				decision, err := decisionService.MakeDecisionForSalesBoost(&goods, &sku, s.apiClient.GetStoreID())
				if err != nil {
					s.logger.WithError(err).Warnf("商品 %s SKU %s 决策失败",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
					stats.FailCount++
					continue
				}

				if decision == nil {
					s.logger.Warnf("商品 %s SKU %s 决策结果为空",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
					stats.FailCount++
					continue
				}

				// 执行决策
				if err := s.executeDecisionForSalesBoost(decision, &goods, &sku); err != nil {
					s.logger.WithError(err).Warnf("商品 %s SKU %s 执行决策失败: %s",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID, decision.Action)
					stats.FailCount++
				} else {
					stats.SuccessCount++
					// 更新统计
					switch decision.Action {
					case models.DecisionAccept:
						stats.AcceptCount++
					case models.DecisionReject:
						stats.RejectCount++
					case models.DecisionReappeal:
						stats.ReappealCount++
					case models.DecisionSkip:
						stats.SkipCount++
					}
				}

				s.logger.Infof("商品 %s SKU %s 决策: %s, 原因: %s",
					goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID,
					decision.Action, decision.Reason)
			}
		}

		// 检查是否处理完所有商品
		if stats.TotalProcessed >= resp.Result.Total {
			break
		}

		pageNo++
	}

	s.logger.Infof("📊 智能核价完成: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d, 成功=%d, 失败=%d",
		stats.TotalProcessed, stats.AcceptCount, stats.RejectCount,
		stats.ReappealCount, stats.SkipCount, stats.SuccessCount, stats.FailCount)

	return stats, nil
}

// executeDecisionForSalesBoost 执行核价决策（新版本，适配销量提升场景）
func (s *AutoPricingService) executeDecisionForSalesBoost(decision *models.PricingDecision, goods *models.SalesBoostGoods, sku *models.SalesBoostSku) error {
	switch decision.Action {
	case models.DecisionAccept:
		// ✅ 接受平台报价
		if sku.TargetSupplierPrice.Amount == "" || sku.TargetSupplierPrice.Currency == "" {
			return fmt.Errorf("目标价格信息不完整")
		}
		skuList := []models.AcceptPriceSkuInfo{
			{
				SkuID:                  sku.SkuID,
				Currency:               sku.TargetSupplierPrice.Currency,
				TargetSupplierPriceStr: sku.TargetSupplierPrice.Amount, // 使用平台推荐的目标价格
			},
		}
		_, err := s.pricingAPI.AcceptPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, skuList, 2)
		return err

	case models.DecisionReject:
		// ❌ 拒绝报价
		skuIDs := []string{sku.SkuID}
		_, err := s.pricingAPI.RejectPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, skuIDs)
		return err

	case models.DecisionReappeal:
		if sku.CurrentSupplierPrice.Amount == "" || sku.TargetSupplierPrice.Amount == "" || sku.CurrentSupplierPrice.Currency == "" {
			return fmt.Errorf("价格信息不完整")
		}
		skuInfoList := []models.ReappealSkuInfo{
			{
				SkuID:                       sku.SkuID,
				SupplierPriceStr:            sku.CurrentSupplierPrice.Amount,
				RecommendedSupplierPriceStr: sku.TargetSupplierPrice.Amount,
				TargetSupplierPriceStr:      fmt.Sprintf("%.2f", decision.AcceptablePrice),
				Currency:                    sku.CurrentSupplierPrice.Currency,
			},
		}
		// 使用 TEMU API 要求的申诉原因枚举值
		appealReasons := []string{"HIGH_COST"}
		_, err := s.pricingAPI.ReappealPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, skuInfoList, 100, appealReasons)
		return err

	case models.DecisionSkip:
		// 跳过，不做任何操作
		s.logger.Info("跳过")
		return nil

	default:
		return fmt.Errorf("未知的决策动作: %s", decision.Action)
	}
}
