// package pricing 提供TEMU平台自动核价服务
package pricing

import (
	"fmt"
	"task-processor/internal/infra/clients/management"
	temuapi "task-processor/internal/temu/api"
	temupricing "task-processor/internal/temu/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingService 自动核价服务
type AutoPricingService struct {
	apiClient      temuapi.APIClientInterface
	pricingService *temupricing.API
	logger         *logrus.Entry
}

// NewAutoPricingService 创建自动核价服务
func NewAutoPricingService(apiClient temuapi.APIClientInterface) *AutoPricingService {
	logger := logrus.WithFields(logrus.Fields{
		"component": "AutoPricingService",
		"storeID":   apiClient.GetStoreID(),
	})

	return &AutoPricingService{
		apiClient:      apiClient,
		pricingService: temupricing.NewAPI(apiClient, logger),
		logger:         logger,
	}
}

// AutoProcessPendingPricesWithRules 根据利润率规则智能处理待核价商品
func (s *AutoPricingService) AutoProcessPendingPricesWithRules(managementClient *management.ClientManager) (*temupricing.Statistics, error) {
	s.logger.Info("开始智能核价处理")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	// 使用基础决策服务
	s.logger.Info("使用基础决策服务处理待核价商品")
	decisionService, err := NewPricingDecisionService(managementClient, s.apiClient.GetStoreID(), nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("创建决策服务失败: %w", err)
	}

	return s.processWithService(decisionService)
}

// AutoProcessPendingPricesWithRulesAndAmazon 根据利润率规则智能处理待核价商品（支持Amazon数据）
func (s *AutoPricingService) AutoProcessPendingPricesWithRulesAndAmazon(
	managementClient *management.ClientManager,
) (*temupricing.Statistics, error) {
	s.logger.Info("开始智能核价处理（Amazon增强版）")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	s.logger.Warn("配置提供者为空，使用基础决策服务")
	return s.AutoProcessPendingPricesWithRules(managementClient)
}

// processWithService 使用指定的决策服务处理待核价商品
func (s *AutoPricingService) processWithService(decisionService DecisionMaker) (*temupricing.Statistics, error) {
	stats := &temupricing.Statistics{}
	pageNo := 1
	pageSize := 25

	for {
		// 获取待核价列表
		resp, err := s.pricingService.GetPendingList(pageNo, pageSize)
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
					case temupricing.DecisionAccept:
						stats.AcceptCount++
					case temupricing.DecisionReject:
						stats.RejectCount++
					case temupricing.DecisionReappeal:
						stats.ReappealCount++
					case temupricing.DecisionSkip:
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
func (s *AutoPricingService) executeDecisionForSalesBoost(decision *temupricing.Decision, goods *temupricing.SalesBoostGoods, sku *temupricing.SalesBoostSku) error {
	switch decision.Action {
	case temupricing.DecisionAccept:
		// ✅ 接受平台报价
		return s.pricingService.AcceptPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, sku)

	case temupricing.DecisionReject:
		// ❌ 拒绝报价
		return s.pricingService.RejectPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, []string{sku.SkuID})

	case temupricing.DecisionSkip:
		// 跳过，不做任何操作
		s.logger.Info("跳过")
		return nil

	default:
		return fmt.Errorf("未知的决策动作: %s", decision.Action)
	}
}
