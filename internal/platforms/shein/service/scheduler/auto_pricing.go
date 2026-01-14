// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/pricing"
	"task-processor/internal/platforms/shein/repo"
	pricingservice "task-processor/internal/platforms/shein/service/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingService 自动核价服务接口
type AutoPricingService interface {
	// FetchPendingPriceProducts 获取待核价产品列表
	FetchPendingPriceProducts(ctx context.Context) ([]pricing.BargainPageData, error)

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
	Product  pricing.BargainPageData
	Action   string // accept, reject, reappeal, skip
	Reason   string
	BatchReq *pricing.BatchHandleCostDiscussRequest
}

// autoPricingServiceImpl 自动核价服务实现
type autoPricingServiceImpl struct {
	managementClient *management.ClientManager
	pricingAPI       repo.PricingAPIInterface
	calculator       *pricingservice.AutoPricingCalculator
	requestBuilder   *PricingRequestBuilder
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
		logger:           logrus.WithField("component", "AutoPricingService"),
	}
}

// FetchPendingPriceProducts 获取待核价产品列表
func (s *autoPricingServiceImpl) FetchPendingPriceProducts(ctx context.Context) ([]pricing.BargainPageData, error) {
	s.logger.Debug("开始获取待核价产品列表")

	var allProducts []pricing.BargainPageData

	// 分页获取所有待处理的议价数据（状态1表示待处理）
	pageNum := 1
	const pageSize = 100

	for {
		req := &pricing.PageRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		// 调用 SHEIN API 获取议价页面数据
		response, err := s.pricingAPI.BargainPage(req, 1)
		if err != nil {
			s.logger.Errorf("获取议价页面数据失败(页面%d): %v", pageNum, err)
			return nil, fmt.Errorf("获取议价页面数据失败: %w", err)
		}

		s.logger.Debugf("页面%d获取到%d个议价数据", pageNum, len(response.Info.Data))

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
		if decision.Action == "skip" || decision.BatchReq == nil {
			stats.SkipCount++
			continue
		}

		stats.TotalProcessed++

		// 调用 SHEIN API 提交核价决策
		_, err := s.pricingAPI.BatchHandleCostDiscuss(decision.BatchReq)
		if err != nil {
			s.logger.Errorf("提交核价决策失败 [%s]: %v", decision.Product.BargainSn, err)
			continue
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
