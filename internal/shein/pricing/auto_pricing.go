// Package pricing 提供 SHEIN 平台定价功能
package pricing

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	shein_pricing "task-processor/internal/shein/api/pricing"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// AutoPricingService 自动核价服务接口
type AutoPricingService interface {
	// FetchPendingPriceProducts 获取待核价产品列表
	FetchPendingPriceProducts(ctx context.Context, startTime, endTime string) ([]shein_pricing.BargainPageData, error)

	// ApplyPricingRules 应用核价规则
	ApplyPricingRules(ctx context.Context, products []shein_pricing.BargainPageData, storeID int64, enableRebargain bool) ([]PricingDecision, error)

	// SubmitPricingResults 提交核价结果
	SubmitPricingResults(ctx context.Context, results []PricingDecision) (*PricingStatistics, error)
}

// PricingStatistics 核价统计信息
type PricingStatistics struct {
	TotalProcessed int // 总处理数
	AcceptCount    int // 接受数
	RejectCount    int // 拒绝数
	ReappealCount  int // 重新报价数
	SkipCount      int // 跳过数
}

// PricingDecision 核价决策
type PricingDecision struct {
	Product      shein_pricing.BargainPageData
	Action       string // accept, reject, reappeal, skip
	Reason       string
	BatchReQuote *shein_pricing.BatchReQuoteRequest // 新接口(所有操作)
}

// autoPricingServiceImpl 自动核价服务实现
type autoPricingServiceImpl struct {
	pricingRuleRepo sheinPricingRuleLister
	mappingRepo     sheinProductImportMappingFinder
	pricingAPI      shein_pricing.PricingAPI
	calculator      *AutoPricingCalculator
	requestBuilder  *PricingRequestBuilder
	timeHelper      *TimeHelper
	logger          *logrus.Entry
}

type sheinPricingRuleLister interface {
	ListByStoreID(ctx context.Context, storeID int64) ([]listingadmin.PricingRule, error)
}

type sheinProductImportMappingFinder interface {
	FindLatest(ctx context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error)
}

// NewAutoPricingService 创建自动核价服务
func NewAutoPricingService(
	pricingRuleRepo sheinPricingRuleLister,
	mappingRepo sheinProductImportMappingFinder,
	pricingAPI shein_pricing.PricingAPI,
) AutoPricingService {
	service := &autoPricingServiceImpl{
		pricingRuleRepo: pricingRuleRepo,
		mappingRepo:     mappingRepo,
		pricingAPI:      pricingAPI,
		calculator:      NewAutoPricingCalculator(),
		requestBuilder:  NewPricingRequestBuilder(),
		timeHelper:      NewTimeHelper(),
		logger:          logger.GetGlobalLogger("AutoPricingService"),
	}
	return service
}

// FetchPendingPriceProducts 获取待核价产品列表
func (s *autoPricingServiceImpl) FetchPendingPriceProducts(ctx context.Context, startTime, endTime string) ([]shein_pricing.BargainPageData, error) {
	// 调整时间范围以符合API限制（不超过三个月）
	adjustedStart, adjustedEnd, err := s.timeHelper.AdjustTimeRangeToLimit(startTime, endTime)
	if err != nil {
		s.logger.Errorf("时间参数处理失败: %v", err)
		return nil, fmt.Errorf("时间参数处理失败: %w", err)
	}

	// 如果调整了时间范围，记录日志
	if adjustedStart != startTime || adjustedEnd != endTime {
		s.logger.Infof("时间范围已调整以符合API限制: %s 到 %s", adjustedStart, adjustedEnd)
	}

	s.logger.WithFields(logrus.Fields{
		"start_time": adjustedStart,
		"end_time":   adjustedEnd,
	}).Debug("开始获取待核价产品列表")

	var allProducts []shein_pricing.BargainPageData

	// 分页获取所有待处理的议价数据（状态1表示待处理）
	pageNum := 1
	const pageSize = 100

	for {
		req := &shein_pricing.PageRequest{
			PageNum:   pageNum,
			PageSize:  pageSize,
			StartTime: adjustedStart,
			EndTime:   adjustedEnd,
		}

		// 调用 SHEIN API 获取议价页面数据
		response, err := s.pricingAPI.BargainPage(req, 1)
		if err != nil {
			s.logger.Errorf("获取议价页面数据失败(页面%d): %v", pageNum, err)
			return nil, fmt.Errorf("获取议价页面数据失败: %w", err)
		}

		s.logger.Infof("页面%d获取到%d个议价数据", pageNum, len(response.Info.Data))

		allProducts = append(allProducts, response.Info.Data...)

		// 如果当前页数据少于页面大小，说明已经到最后一页
		if len(response.Info.Data) < pageSize {
			break
		}
		pageNum++
	}

	s.logger.Infof("获取待核价产品列表完成，共%d个产品", len(allProducts))
	return allProducts, nil
}

// ApplyPricingRules 应用核价规则
func (s *autoPricingServiceImpl) ApplyPricingRules(ctx context.Context, products []shein_pricing.BargainPageData, storeID int64, enableRebargain bool) ([]PricingDecision, error) {
	s.logger.WithFields(logrus.Fields{
		"store_id":         storeID,
		"count":            len(products),
		"enable_rebargain": enableRebargain,
	}).Debug("开始应用核价规则")

	// 获取核价规则
	rules, err := s.getPricingRules(storeID)
	if err != nil {
		return nil, fmt.Errorf("获取核价规则失败: %w", err)
	}

	s.logger.Infof("获取到%d条核价规则", len(rules))

	// 对每个产品应用规则
	decisions := make([]PricingDecision, 0, len(products))
	for _, product := range products {
		decision := s.evaluateProduct(&product, rules, storeID, enableRebargain)
		decisions = append(decisions, decision)
	}

	s.logger.Debug("应用核价规则完成")
	return decisions, nil
}

// getPricingRules 获取核价规则
func (s *autoPricingServiceImpl) getPricingRules(storeID int64) ([]listingruntime.PricingRule, error) {
	if s.pricingRuleRepo == nil {
		return nil, fmt.Errorf("pricing rule repository is not configured")
	}
	rules, err := s.pricingRuleRepo.ListByStoreID(context.Background(), storeID)
	if err != nil {
		return nil, fmt.Errorf("通过本地仓储获取SHEIN定价规则失败: %w", err)
	}
	items := make([]listingruntime.PricingRule, 0, len(rules))
	for i := range rules {
		items = append(items, sheinPricingRuleFromListingRule(rules[i]))
	}
	return items, nil
}

func sheinPricingRuleFromListingRule(rule listingadmin.PricingRule) listingruntime.PricingRule {
	dto := listingruntime.PricingRule{
		ID:         rule.ID,
		Name:       rule.Name,
		RuleCode:   rule.RuleCode,
		StoreID:    rule.StoreID,
		CategoryID: rule.CategoryID,
		RuleType:   rule.RuleType,
		Status:     int(rule.Status),
		TenantID:   rule.TenantID,
	}
	dto.PriceMin = sheinPtrFloat64(rule.PriceMin)
	dto.PriceMax = sheinPtrFloat64(rule.PriceMax)
	dto.RuleValue = sheinPtrFloat64(rule.RuleValue)
	dto.FixedValue = rule.FixedValue
	if rule.Description != "" {
		dto.Description = sheinPtrString(rule.Description)
	}
	if rule.AcceptCondition != "" {
		dto.AcceptCondition = sheinPtrString(rule.AcceptCondition)
	}
	if rule.RejectCondition != "" {
		dto.RejectCondition = sheinPtrString(rule.RejectCondition)
	}
	if rule.Remark != "" {
		dto.Remark = sheinPtrString(rule.Remark)
	}
	return dto
}

func sheinPtrString(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func sheinPtrFloat64(value float64) *float64 {
	out := value
	return &out
}

func sheinProductImportMappingFromListing(mapping *listingadmin.ProductImportMapping) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	return &listingruntime.ProductImportMapping{
		ID:                      mapping.ID,
		ImportTaskID:            mapping.ImportTaskID,
		StoreID:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductID:               mapping.ProductID,
		ParentProductID:         sheinPtrString(mapping.ParentProductID),
		SKU:                     sheinPtrString(mapping.SKU),
		PlatformProductID:       sheinPtrString(mapping.PlatformProductID),
		PlatformParentProductID: sheinPtrString(mapping.PlatformParentProductID),
		CostPrice:               sheinFloat64Value(mapping.CostPrice),
		FilterRuleID:            sheinInt64Value(mapping.FilterRuleID),
		FilterRuleRange:         sheinPtrString(mapping.FilterRuleRange),
		ProfitRuleID:            sheinInt64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     sheinPtrFloat64(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: sheinPtrFloat64(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		Remark:                  sheinPtrString(mapping.Remark),
		TenantID:                mapping.TenantID,
	}
}

func sheinFloat64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func sheinInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// SubmitPricingResults 提交核价结果
func (s *autoPricingServiceImpl) SubmitPricingResults(ctx context.Context, results []PricingDecision) (*PricingStatistics, error) {
	s.logger.WithField("count", len(results)).Debug("开始提交核价结果")

	stats := &PricingStatistics{
		TotalProcessed: 0,
		AcceptCount:    0,
		RejectCount:    0,
		ReappealCount:  0,
		SkipCount:      0,
	}

	for _, decision := range results {
		// 跳过不需要提交的决策
		if decision.Action == "skip" || decision.BatchReQuote == nil {
			stats.SkipCount++
			continue
		}

		stats.TotalProcessed++

		// 使用新接口提交所有核价决策
		_, err := s.pricingAPI.BatchReQuote(decision.BatchReQuote)
		if err != nil {
			s.logger.Warnf("新接口提交失败 [%s],尝试使用旧接口: %v", decision.Product.BargainSn, err)

			// 降级到旧接口
			if fallbackErr := s.fallbackToLegacyAPI(&decision); fallbackErr != nil {
				s.logger.Errorf("旧接口提交也失败 [%s]: %v", decision.Product.BargainSn, fallbackErr)
				continue
			}
			s.logger.Infof("旧接口提交成功 [%s]", decision.Product.BargainSn)
		}

		// 统计各类操作的数量
		switch decision.Action {
		case "accept":
			stats.AcceptCount++
			s.logger.Infof("接受核价 [%s]: %s", decision.Product.BargainSn, decision.Reason)
		case "reject":
			stats.RejectCount++
			s.logger.Infof("拒绝核价 [%s]: %s", decision.Product.BargainSn, decision.Reason)
		case "reappeal":
			stats.ReappealCount++
			s.logger.Infof("重新议价 [%s]: %s", decision.Product.BargainSn, decision.Reason)
		}
	}

	s.logger.Infof("提交核价结果完成: 总数=%d, 接受=%d, 拒绝=%d, 重新议价=%d, 跳过=%d",
		stats.TotalProcessed, stats.AcceptCount, stats.RejectCount, stats.ReappealCount, stats.SkipCount)
	return stats, nil
}

// fallbackToLegacyAPI 降级到旧接口提交核价决策
func (s *autoPricingServiceImpl) fallbackToLegacyAPI(decision *PricingDecision) error {
	var legacyRequest *shein_pricing.BatchHandleCostDiscussRequest

	// 根据操作类型构建旧接口请求
	switch decision.Action {
	case "accept":
		legacyRequest = s.requestBuilder.BuildLegacyAcceptRequest(&decision.Product)
	case "reject":
		legacyRequest = s.requestBuilder.BuildLegacyRejectRequest(&decision.Product)
	case "reappeal":
		// 旧接口不支持重新议价,跳过
		s.logger.Warnf("旧接口不支持重新议价操作,跳过 [%s]", decision.Product.BargainSn)
		return fmt.Errorf("旧接口不支持重新议价操作")
	default:
		return fmt.Errorf("未知的操作类型: %s", decision.Action)
	}

	// 调用旧接口
	_, err := s.pricingAPI.BatchHandleCostDiscuss(legacyRequest)
	if err != nil {
		return fmt.Errorf("旧接口调用失败: %w", err)
	}

	return nil
}
