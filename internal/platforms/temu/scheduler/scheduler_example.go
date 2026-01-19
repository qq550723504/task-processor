// Package scheduler 提供TEMU定时任务使用示例
package scheduler

import (
	"context"
	"time"

	schedulerservice "task-processor/internal/platforms/temu/service/scheduler"

	"github.com/sirupsen/logrus"
)

// StartTemuProductSync 启动TEMU产品同步定时任务的示例
func StartTemuProductSync(ctx context.Context) error {
	logger := logrus.WithField("component", "TemuSchedulerExample")

	// 1. 创建TEMU API客户端（需要根据实际情况配置）
	// apiClient := client.NewAPIClient(config)
	// productAPI := services.NewProductAPI(apiClient, logger)

	// 2. 创建管理系统客户端（需要根据实际情况配置）
	// managementClient := management.NewClientManager(config)

	// 3. 创建服务工厂
	// serviceFactory := schedulerservice.NewServiceFactory(
	//     managementClient,
	//     productAPI,
	//     managementClient.GetProductImportMappingClient(),
	//     managementClient.GetStoreClient(),
	// )

	// 4. 创建同步调度器
	// scheduler := NewSyncScheduler(
	//     managementClient,
	//     productAPI,
	//     serviceFactory,
	// )

	// 5. 启动定时任务（每30分钟同步一次）
	// syncInterval := 30 * time.Minute
	// return scheduler.Start(ctx, syncInterval)

	logger.Info("TEMU产品同步定时任务示例 - 请根据实际配置启用")
	return nil
}

// CreateTemuProductSyncTask 创建单次TEMU产品同步任务的示例
func CreateTemuProductSyncTask(ctx context.Context, tenantID, storeID int64) error {
	logger := logrus.WithField("component", "TemuTaskExample")

	// 示例配置
	config := &schedulerservice.ProductSyncConfig{
		PageSize:        50, // 每页50个产品
		MaxPages:        10, // 最多获取10页
		Language:        "en",
		IncludeInactive: false, // 只同步已上架的产品
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
		"config":    config,
	}).Info("TEMU产品同步任务配置示例")

	// 实际使用时的步骤：
	// 1. 创建ProductSyncService
	// 2. 调用FetchProductList获取产品
	// 3. 调用ConvertProducts转换格式
	// 4. 调用SaveProducts保存到管理系统

	return nil
}

// SchedulerConfig TEMU调度器配置
type SchedulerConfig struct {
	// 同步间隔
	SyncInterval time.Duration `json:"sync_interval"`

	// 产品同步配置
	ProductSync *schedulerservice.ProductSyncConfig `json:"product_sync"`

	// 是否启用自动同步
	EnableAutoSync bool `json:"enable_auto_sync"`

	// 同步时间窗口（可选，格式：HH:MM-HH:MM）
	SyncTimeWindow string `json:"sync_time_window"`

	// 最大并发数
	MaxConcurrency int `json:"max_concurrency"`
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		SyncInterval: 30 * time.Minute,
		ProductSync: &schedulerservice.ProductSyncConfig{
			PageSize:        100,
			MaxPages:        0, // 不限制页数
			Language:        "en",
			IncludeInactive: false,
		},
		EnableAutoSync: true,
		SyncTimeWindow: "", // 全天候同步
		MaxConcurrency: 3,  // 最多3个并发任务
	}
}
