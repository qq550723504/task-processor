// Package scheduler 提供TEMU平台自动核价任务适配器
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/infra/clients/management"
	commonscheduler "task-processor/internal/platforms/common/scheduler"
	"task-processor/internal/platforms/temu/api"
	temupricing "task-processor/internal/platforms/temu/api/pricing"
	"task-processor/internal/platforms/temu/pricingsvc"

	"github.com/sirupsen/logrus"
)

// TemuAutoPricingAdapter 适配Temu的自动核价服务到通用接口
// Temu平台的特点是一次性完成获取、决策和提交的完整流程
type TemuAutoPricingAdapter struct {
	autoPricingService *pricingsvc.AutoPricingService
	managementClient   *management.ClientManager
	logger             *logrus.Entry
}

// NewTemuAutoPricingAdapter 创建Temu自动核价适配器
func NewTemuAutoPricingAdapter(
	apiClient api.APIClientInterface,
	managementClient *management.ClientManager,
) *TemuAutoPricingAdapter {
	return &TemuAutoPricingAdapter{
		autoPricingService: pricingsvc.NewAutoPricingService(apiClient),
		managementClient:   managementClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuAutoPricingAdapter",
			"store_id":  apiClient.GetStoreID(),
		}),
	}
}

// FetchPendingPriceProducts 获取待核价产品列表
// Temu平台的实现是在processWithService中分页获取，这里返回空切片
// 实际的获取和处理逻辑在SubmitPricingResults中完成
func (a *TemuAutoPricingAdapter) FetchPendingPriceProducts(ctx context.Context, startDate, endDate string) ([]interface{}, error) {
	a.logger.Debug("Temu平台的待核价产品获取在处理流程中完成")
	// 返回空切片，表示需要在后续步骤中获取
	return []interface{}{}, nil
}

// ApplyPricingRules 应用核价规则
// Temu平台的规则应用是在决策服务中完成的，这里直接返回输入
func (a *TemuAutoPricingAdapter) ApplyPricingRules(ctx context.Context, products []interface{}, storeID int64, enableRebargain bool) ([]interface{}, error) {
	a.logger.Debug("Temu平台的核价规则应用在决策服务中完成")
	// 直接返回输入，实际的规则应用在SubmitPricingResults中完成
	return products, nil
}

// SubmitPricingResults 提交核价结果
// Temu平台的实现是一次性完成获取、决策和提交的完整流程
func (a *TemuAutoPricingAdapter) SubmitPricingResults(ctx context.Context, results []interface{}) (*commonscheduler.PricingStats, error) {
	a.logger.Info("开始Temu自动核价处理（完整流程）")

	// 调用Temu的自动核价服务，它会完成：
	// 1. 分页获取待核价商品
	// 2. 应用核价规则做出决策
	// 3. 提交核价结果到平台
	stats, err := a.autoPricingService.AutoProcessPendingPricesWithRules(a.managementClient)
	if err != nil {
		return nil, fmt.Errorf("Temu自动核价处理失败: %w", err)
	}

	// 转换统计信息到通用格式
	return convertTemuStats(stats), nil
}

// convertTemuStats 转换Temu的统计信息到通用格式
func convertTemuStats(stats *temupricing.Statistics) *commonscheduler.PricingStats {
	if stats == nil {
		return &commonscheduler.PricingStats{}
	}

	return &commonscheduler.PricingStats{
		TotalProcessed: stats.TotalProcessed,
		AcceptCount:    stats.AcceptCount,
		RejectCount:    stats.RejectCount,
		ReappealCount:  stats.ReappealCount,
		SkipCount:      stats.SkipCount,
	}
}
