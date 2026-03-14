// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/api/pricing"
	"task-processor/internal/platforms/shein/repo"
	pricingservice "task-processor/internal/platforms/shein/service/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingService 自动核价服务接口
type AutoPricingService interface {
	// FetchPendingPriceProducts 获取待核价产品列表
	FetchPendingPriceProducts(ctx context.Context, startTime, endTime string) ([]pricing.BargainPageData, error)

	// ApplyPricingRules 应用核价规则
	ApplyPricingRules(ctx context.Context, products []pricing.BargainPageData, storeID int64, enableRebargain bool) ([]PricingDecision, error)

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
	Product      pricing.BargainPageData
	Action       string // accept, reject, reappeal, skip
	Reason       string
	BatchReQuote *pricing.BatchReQuoteRequest // 新接口(所有操作)
}

// autoPricingServiceImpl 自动核价服务实现
type autoPricingServiceImpl struct {
	managementClient *management.ClientManager
	pricingAPI       repo.PricingAPIInterface
	calculator       *pricingservice.AutoPricingCalculator
	requestBuilder   *PricingRequestBuilder
	timeHelper       *shein.TimeHelper
	logger           *logrus.Entry
}

// NewAutoPricingService 创建自动核价服务
func NewAutoPricingService(
	managementClient *management.ClientManager,
	pricingAPI repo.PricingAPIInterface,
) AutoPricingService {
	return &autoPricingServiceImpl{
		managementClient: managementClient,
		pricingAPI:       pricingAPI,
		calculator:       pricingservice.NewAutoPricingCalculator(),
		requestBuilder:   NewPricingRequestBuilder(),
		timeHelper:       shein.NewTimeHelper(),
		logger:           logrus.WithField("component", "AutoPricingService"),
	}
}

// FetchPendingPriceProducts 获取待核价产品列表
func (s *autoPricingServiceImpl) FetchPendingPriceProducts(ctx context.Context, startTime, endTime string) ([]pricing.BargainPageData, error) {
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

	var allProducts []pricing.BargainPageData

	// 分页获取所有待处理的议价数据（状态1表示待处理）
	pageNum := 1
	const pageSize = 100

	for {
		req := &pricing.PageRequest{
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
func (s *autoPricingServiceImpl) ApplyPricingRules(ctx context.Context, products []pricing.BargainPageData, storeID int64, enableRebargain bool) ([]PricingDecision, error) {
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
func (s *autoPricingServiceImpl) getPricingRules(storeID int64) ([]managementapi.PricingRuleRespDTO, error) {
	pricingRuleAPI := s.managementClient.GetPricingRuleClient()
	reqDto := &managementapi.PricingRuleReqDTO{
		StoreID: &storeID,
	}

	rules, err := pricingRuleAPI.GetPricingRule(reqDto)
	if err != nil {
		return nil, fmt.Errorf("获取定价规则失败: %w", err)
	}

	return rules, nil
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
	var legacyRequest *pricing.BatchHandleCostDiscussRequest

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
