// Package temu 提供TEMU平台与通用核价系统的集成。
package client

import (
	"context"
	"fmt"
	"task-processor/common/management"
	"task-processor/common/pricing"
	"task-processor/common/pricing/model"
	"task-processor/common/pricing/service"

	"github.com/sirupsen/logrus"
)

// IntegratedPricingService TEMU集成核价服务
type IntegratedPricingService struct {
	commonService service.CommonPricingService
	temuAdapter   *pricing.TemuAdapter
	tenantID      int64
	storeID       int64
	logger        *logrus.Entry
}

// NewIntegratedPricingService 创建TEMU集成核价服务
func NewIntegratedPricingService(managementClient *management.ClientManager, tenantID, storeID int64) *IntegratedPricingService {
	// 创建服务工厂
	factory := pricing.NewServiceFactory(managementClient)

	// 创建通用核价服务
	commonService := factory.CreateCommonPricingService()

	// 获取TEMU适配器
	temuAdapter := factory.CreateTemuAdapter()

	return &IntegratedPricingService{
		commonService: commonService,
		temuAdapter:   temuAdapter,
		tenantID:      tenantID,
		storeID:       storeID,
		logger: logrus.WithFields(logrus.Fields{
			"component": "IntegratedPricingService",
			"platform":  "temu",
			"tenantID":  tenantID,
			"storeID":   storeID,
		}),
	}
}

// MakeDecision 使用新系统对单个商品做出核价决策
func (s *IntegratedPricingService) MakeDecision(ctx context.Context, item *Sku, storeId int64) (*PricingDecision, error) {
	// 转换为通用核价上下文
	pricingCtx := s.temuAdapter.ConvertFromTemuSku(item, storeId, s.tenantID)
	pricingCtx.Ctx = ctx

	// 加载店铺配置和核价规则
	if err := s.loadContextData(ctx, pricingCtx); err != nil {
		s.logger.Errorf("加载上下文数据失败: %v", err)
		return s.createSkipDecision(item, fmt.Sprintf("加载上下文数据失败: %v", err)), nil
	}

	// 使用通用核价服务处理
	result, err := s.commonService.ProcessSingleProduct(ctx, pricingCtx)
	if err != nil {
		s.logger.Errorf("通用核价服务处理失败: %v", err)
		return s.createSkipDecision(item, fmt.Sprintf("核价处理失败: %v", err)), nil
	}

	// 转换回TEMU格式
	return s.convertToTemuDecision(item, result), nil
}

// MakeDecisionForSalesBoost 使用新系统对销量提升场景做出核价决策
func (s *IntegratedPricingService) MakeDecisionForSalesBoost(ctx context.Context, goods *SalesBoostGoods, sku *SalesBoostSku, storeId int64) (*PricingDecision, error) {
	// 转换为通用核价上下文
	pricingCtx := s.temuAdapter.ConvertFromTemuSalesBoost(goods, sku, storeId, s.tenantID)
	pricingCtx.Ctx = ctx

	// 加载店铺配置和核价规则
	if err := s.loadContextData(ctx, pricingCtx); err != nil {
		s.logger.Errorf("加载上下文数据失败: %v", err)
		return s.createSkipDecisionForSalesBoost(fmt.Sprintf("加载上下文数据失败: %v", err)), nil
	}

	// 使用通用核价服务处理
	result, err := s.commonService.ProcessSingleProduct(ctx, pricingCtx)
	if err != nil {
		s.logger.Errorf("通用核价服务处理失败: %v", err)
		return s.createSkipDecisionForSalesBoost(fmt.Sprintf("核价处理失败: %v", err)), nil
	}

	// 转换回TEMU格式
	return s.convertToTemuDecisionForSalesBoost(result), nil
}

// loadContextData 加载核价上下文数据
func (s *IntegratedPricingService) loadContextData(ctx context.Context, pricingCtx *model.PricingContext) error {
	// 加载店铺配置
	storeConfig, err := s.loadStoreConfig(ctx, pricingCtx.StoreID)
	if err != nil {
		return fmt.Errorf("加载店铺配置失败: %w", err)
	}
	pricingCtx.StoreConfig = storeConfig

	// 加载核价规则
	pricingRules, err := s.loadPricingRules(ctx, pricingCtx.StoreID)
	if err != nil {
		s.logger.Warnf("加载核价规则失败: %v，将使用默认规则", err)
		// 使用默认规则
		pricingRules = []model.PricingRule{
			{
				Name:      "默认利润率规则",
				RuleType:  model.RuleTypePercent,
				RuleValue: floatPtr(0.5), // 50%利润率
				Status:    0,
			},
		}
	}
	pricingCtx.PricingRules = pricingRules

	return nil
}

// loadStoreConfig 加载店铺配置
func (s *IntegratedPricingService) loadStoreConfig(ctx context.Context, storeID int64) (*model.StoreConfig, error) {
	// 这里应该调用管理端API获取店铺配置
	// 暂时返回默认配置
	return &model.StoreConfig{
		ID:                      storeID,
		Name:                    "TEMU店铺",
		EnableAutoPrice:         boolPtr(true),
		EnableRebargain:         boolPtr(true),
		TemuPriceRejectStrategy: "KEEP_ONLINE",
	}, nil
}

// loadPricingRules 加载核价规则
func (s *IntegratedPricingService) loadPricingRules(ctx context.Context, storeID int64) ([]model.PricingRule, error) {
	// 这里应该调用管理端API获取核价规则
	// 暂时返回默认规则
	return []model.PricingRule{
		{
			ID:        1,
			Name:      "TEMU默认规则",
			RuleType:  model.RuleTypePercent,
			RuleValue: floatPtr(0.5), // 50%利润率
			Status:    0,
		},
	}, nil
}

// convertToTemuDecision 转换为TEMU决策格式
func (s *IntegratedPricingService) convertToTemuDecision(item *Sku, result *model.PricingResult) *PricingDecision {
	decision := &PricingDecision{
		Sku:    item,
		Reason: result.Reason,
	}

	// 转换决策动作
	switch result.Action {
	case model.ActionAccept:
		decision.Action = DecisionAccept
	case model.ActionReject:
		decision.Action = DecisionReject
	case model.ActionReappeal:
		decision.Action = DecisionReappeal
	case model.ActionSkip:
		decision.Action = DecisionSkip
	default:
		decision.Action = DecisionSkip
		decision.Reason = fmt.Sprintf("未知决策类型: %s", result.Action)
	}

	return decision
}

// convertToTemuDecisionForSalesBoost 转换销量提升场景的决策格式
func (s *IntegratedPricingService) convertToTemuDecisionForSalesBoost(result *model.PricingResult) *PricingDecision {
	decision := &PricingDecision{
		Reason:          result.Reason,
		ProfitMargin:    result.ProfitMargin,
		TargetPrice:     result.SuggestPrice,
		TargetMargin:    result.TargetMargin,
		MinMargin:       result.MinMargin,
		AcceptablePrice: result.AcceptablePrice,
	}

	// 转换决策动作
	switch result.Action {
	case model.ActionAccept:
		decision.Action = DecisionAccept
	case model.ActionReject:
		decision.Action = DecisionReject
	case model.ActionReappeal:
		decision.Action = DecisionReappeal
	case model.ActionSkip:
		decision.Action = DecisionSkip
	default:
		decision.Action = DecisionSkip
		decision.Reason = fmt.Sprintf("未知决策类型: %s", result.Action)
	}

	return decision
}

// createSkipDecision 创建跳过决策
func (s *IntegratedPricingService) createSkipDecision(item *Sku, reason string) *PricingDecision {
	return &PricingDecision{
		Sku:    item,
		Action: DecisionSkip,
		Reason: reason,
	}
}

// createSkipDecisionForSalesBoost 创建销量提升场景的跳过决策
func (s *IntegratedPricingService) createSkipDecisionForSalesBoost(reason string) *PricingDecision {
	return &PricingDecision{
		Action: DecisionSkip,
		Reason: reason,
	}
}

// 辅助函数
func boolPtr(b bool) *bool {
	return &b
}

func floatPtr(f float64) *float64 {
	return &f
}
