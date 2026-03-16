// Package scheduler 提供平台通用的自动核价任务基础实现
// 用于半托管模式电商平台的自动核价功能
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

// PricingStats 核价统计信息
type PricingStats struct {
	TotalProcessed int // 总处理数
	AcceptCount    int // 接受数
	RejectCount    int // 拒绝数
	ReappealCount  int // 重新议价/报价数
	SkipCount      int // 跳过数
}

// AutoPricingService 自动核价服务接口
// 各平台需要实现此接口
type AutoPricingService interface {
	// FetchPendingPriceProducts 获取待核价产品列表
	FetchPendingPriceProducts(ctx context.Context, startDate, endDate string) ([]any, error)

	// ApplyPricingRules 应用核价规则
	ApplyPricingRules(ctx context.Context, products []any, storeID int64, enableRebargain bool) ([]any, error)

	// SubmitPricingResults 提交核价结果到平台
	SubmitPricingResults(ctx context.Context, results []any) (*PricingStats, error)
}

// AutoPricingTask 通用自动核价任务
// 用于半托管模式电商平台的价格审核和自动核价
type AutoPricingTask struct {
	*BaseTask
	managementClient *management.ClientManager
	pricingService   AutoPricingService
	logger           *logrus.Entry
	platformName     string
}

// AutoPricingTaskConfig 自动核价任务配置
type AutoPricingTaskConfig struct {
	TaskConfig       appscheduler.TaskConfig
	ManagementClient *management.ClientManager
	PricingService   AutoPricingService
	PlatformName     string
}

// NewAutoPricingTask 创建通用自动核价任务
func NewAutoPricingTask(config AutoPricingTaskConfig) *AutoPricingTask {
	baseTask := NewBaseTask(config.TaskConfig)

	return &AutoPricingTask{
		BaseTask:         baseTask,
		managementClient: config.ManagementClient,
		pricingService:   config.PricingService,
		platformName:     config.PlatformName,
		logger: logrus.WithFields(logrus.Fields{
			"component": fmt.Sprintf("%sAutoPricingTask", config.PlatformName),
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TaskConfig.TenantID,
			"store_id":  config.TaskConfig.StoreID,
		}),
	}
}

// Execute 执行自动核价任务
func (t *AutoPricingTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Infof("开始执行%s自动核价任务", t.platformName)

	// 1. 检查店铺是否启用自动核价
	storeInfo, err := t.managementClient.GetStoreClient().GetStore(t.GetStoreID())
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice {
		t.logger.Infof("店铺 %s 未启用自动核价功能，跳过", storeInfo.Name)
		return nil
	}

	// 2. 获取待核价产品列表（使用默认时间范围）
	pendingProducts, err := t.pricingService.FetchPendingPriceProducts(ctx, "", "")
	if err != nil {
		return fmt.Errorf("获取%s待核价产品失败: %w", t.platformName, err)
	}

	t.logger.Infof("获取到 %d 个%s待核价产品", len(pendingProducts), t.platformName)

	if len(pendingProducts) == 0 {
		t.logger.Infof("没有%s待核价产品", t.platformName)
		return nil
	}

	// 3. 获取是否启用重新议价配置
	enableRebargain := false
	if storeInfo.EnableRebargain != nil {
		enableRebargain = *storeInfo.EnableRebargain
	}

	// 4. 应用核价规则
	pricingResults, err := t.pricingService.ApplyPricingRules(ctx, pendingProducts, t.GetStoreID(), enableRebargain)
	if err != nil {
		return fmt.Errorf("应用%s核价规则失败: %w", t.platformName, err)
	}

	// 5. 提交核价结果到平台
	stats, err := t.pricingService.SubmitPricingResults(ctx, pricingResults)
	if err != nil {
		return fmt.Errorf("提交%s核价结果失败: %w", t.platformName, err)
	}

	// 6. 记录统计信息
	t.logger.WithFields(logrus.Fields{
		"total_processed": stats.TotalProcessed,
		"accept_count":    stats.AcceptCount,
		"reject_count":    stats.RejectCount,
		"reappeal_count":  stats.ReappealCount,
		"skip_count":      stats.SkipCount,
	}).Infof("%s自动核价任务执行完成", t.platformName)

	return nil
}

// GetManagementClient 获取管理客户端
func (t *AutoPricingTask) GetManagementClient() *management.ClientManager {
	return t.managementClient
}

// GetPricingService 获取核价服务
func (t *AutoPricingTask) GetPricingService() AutoPricingService {
	return t.pricingService
}
