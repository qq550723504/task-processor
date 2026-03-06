// Package scheduler 提供平台通用的产品同步任务基础实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// ProductSyncService 产品同步服务接口
// 各平台需要实现此接口
type ProductSyncService interface {
	// FetchProductList 从平台API获取产品列表
	FetchProductList(ctx context.Context) ([]interface{}, error)

	// ConvertProducts 转换产品为后端格式
	ConvertProducts(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error)

	// SaveProducts 批量保存产品到管理系统
	SaveProducts(ctx context.Context, products []interface{}) (int, error)
}

// ProductSyncTask 通用产品同步任务
type ProductSyncTask struct {
	*BaseTask
	managementClient *management.ClientManager
	syncService      ProductSyncService
	logger           *logrus.Entry
	platformName     string
}

// ProductSyncTaskConfig 产品同步任务配置
type ProductSyncTaskConfig struct {
	TaskConfig       appscheduler.TaskConfig
	ManagementClient *management.ClientManager
	SyncService      ProductSyncService
	PlatformName     string
}

// NewProductSyncTask 创建通用产品同步任务
func NewProductSyncTask(config ProductSyncTaskConfig) *ProductSyncTask {
	baseTask := NewBaseTask(config.TaskConfig)

	return &ProductSyncTask{
		BaseTask:         baseTask,
		managementClient: config.ManagementClient,
		syncService:      config.SyncService,
		platformName:     config.PlatformName,
		logger: logrus.WithFields(logrus.Fields{
			"component": fmt.Sprintf("%sProductSyncTask", config.PlatformName),
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TaskConfig.TenantID,
			"store_id":  config.TaskConfig.StoreID,
		}),
	}
}

// Execute 执行产品同步任务
func (t *ProductSyncTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Infof("开始执行%s产品同步任务", t.platformName)

	// 1. 从平台API获取产品列表
	products, err := t.syncService.FetchProductList(ctx)
	if err != nil {
		return fmt.Errorf("获取%s产品列表失败: %w", t.platformName, err)
	}

	t.logger.Infof("获取到 %d 个%s产品", len(products), t.platformName)

	// 2. 转换为后端格式
	productDataList, err := t.syncService.ConvertProducts(ctx, products, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("转换%s产品格式失败: %w", t.platformName, err)
	}

	// 3. 批量保存到管理系统
	savedCount, err := t.syncService.SaveProducts(ctx, productDataList)
	if err != nil {
		return fmt.Errorf("保存%s产品失败: %w", t.platformName, err)
	}

	// 4. 记录同步统计
	t.logger.Infof("%s产品同步任务执行完成，成功同步 %d 个产品", t.platformName, savedCount)
	return nil
}

// GetManagementClient 获取管理客户端
func (t *ProductSyncTask) GetManagementClient() *management.ClientManager {
	return t.managementClient
}

// GetSyncService 获取同步服务
func (t *ProductSyncTask) GetSyncService() ProductSyncService {
	return t.syncService
}
