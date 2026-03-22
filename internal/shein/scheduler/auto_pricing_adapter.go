// package scheduler 提供SHEIN平台自动核价任务适配器
package scheduler

import (
	"context"

	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/shein/api/pricing"
	sheinpricing "task-processor/internal/shein/pricing"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// SheinAutoPricingAdapter 适配Shein的自动核价服务到通用接口
type SheinAutoPricingAdapter struct {
	pricingService sheinpricing.AutoPricingService
	logger         *logrus.Entry
}

// NewSheinAutoPricingAdapter 创建Shein自动核价适配器
func NewSheinAutoPricingAdapter(pricingService sheinpricing.AutoPricingService) *SheinAutoPricingAdapter {
	return &SheinAutoPricingAdapter{
		pricingService: pricingService,
		logger: logger.GetGlobalLogger("SheinAutoPricingAdapter"),
	}
}

// FetchPendingPriceProducts 获取待核价产品列表
func (a *SheinAutoPricingAdapter) FetchPendingPriceProducts(ctx context.Context, startDate, endDate string) ([]any, error) {
	a.logger.Debug("开始获取Shein待核价产品列表")

	// 调用Shein的服务获取待核价产品
	products, err := a.pricingService.FetchPendingPriceProducts(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]any, len(products))
	for i, p := range products {
		result[i] = p
	}

	return result, nil
}

// ApplyPricingRules 应用核价规则
func (a *SheinAutoPricingAdapter) ApplyPricingRules(ctx context.Context, products []any, storeID int64, enableRebargain bool) ([]any, error) {
	a.logger.Debug("开始应用Shein核价规则")

	// 转换回Shein的产品类型
	sheinProducts := make([]pricing.BargainPageData, len(products))
	for i, p := range products {
		if product, ok := p.(pricing.BargainPageData); ok {
			sheinProducts[i] = product
		}
	}

	// 调用Shein的服务应用核价规则
	decisions, err := a.pricingService.ApplyPricingRules(ctx, sheinProducts, storeID, enableRebargain)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]any, len(decisions))
	for i, d := range decisions {
		result[i] = d
	}

	return result, nil
}

// SubmitPricingResults 提交核价结果
func (a *SheinAutoPricingAdapter) SubmitPricingResults(ctx context.Context, results []any) (*platformtask.PricingStats, error) {
	a.logger.Debug("开始提交Shein核价结果")

	// 转换回Shein的决策类型
	sheinDecisions := make([]sheinpricing.PricingDecision, len(results))
	for i, r := range results {
		if decision, ok := r.(sheinpricing.PricingDecision); ok {
			sheinDecisions[i] = decision
		}
	}

	// 调用Shein的服务提交核价结果
	stats, err := a.pricingService.SubmitPricingResults(ctx, sheinDecisions)
	if err != nil {
		return nil, err
	}

	// 转换为通用统计格式
	return convertSheinStats(stats), nil
}

// convertSheinStats 转换Shein的统计信息到通用格式
func convertSheinStats(stats *sheinpricing.PricingStatistics) *platformtask.PricingStats {
	if stats == nil {
		return &platformtask.PricingStats{}
	}

	return &platformtask.PricingStats{
		TotalProcessed: stats.TotalProcessed,
		AcceptCount:    stats.AcceptCount,
		RejectCount:    stats.RejectCount,
		ReappealCount:  stats.ReappealCount,
		SkipCount:      stats.SkipCount,
	}
}
