package scheduler

import (
	"fmt"

	"task-processor/common/amazon"
	"task-processor/common/management"
	shops "task-processor/common/shein"
	"task-processor/platforms/common"
	"task-processor/platforms/shein"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// MonitorScheduler 产品监控调度器
type MonitorScheduler struct {
	cron            *cron.Cron
	sheinMonitors   map[int64]*shein.SheinProductMonitorService
	sheinAPIClients map[int64]*shops.ShopAPIClient
	clientManager   *management.ClientManager
	amazonProcessor *amazon.AmazonProcessor
	monitorConfig   *common.MonitorConfig
	eventHandler    common.MonitorEventHandler
}

// NewMonitorScheduler 创建监控调度器
func NewMonitorScheduler(
	clientManager *management.ClientManager,
	amazonProcessor *amazon.AmazonProcessor,
	monitorConfig *common.MonitorConfig,
	eventHandler common.MonitorEventHandler,
) *MonitorScheduler {
	return &MonitorScheduler{
		cron:            cron.New(),
		sheinMonitors:   make(map[int64]*shein.SheinProductMonitorService),
		sheinAPIClients: make(map[int64]*shops.ShopAPIClient),
		clientManager:   clientManager,
		amazonProcessor: amazonProcessor,
		monitorConfig:   monitorConfig,
		eventHandler:    eventHandler,
	}
}

// RegisterSheinStore 注册 SHEIN 店铺监控
func (s *MonitorScheduler) RegisterSheinStore(storeID int64, apiClient *shops.ShopAPIClient) {
	s.sheinAPIClients[storeID] = apiClient
	tenantID := apiClient.GetTenantID()

	// 使用公共方法创建同步服务（监控服务需要用到）
	syncService := createSheinSyncService(s.clientManager)

	// 创建监控服务
	monitorService := shein.NewSheinProductMonitorService(
		s.monitorConfig,
		s.clientManager.GetProductImportMappingClient(),
		s.eventHandler,
		syncService,
		s.amazonProcessor,
		s.clientManager.GetRawJsonDataClient(),
		s.clientManager.GetInventoryRecordClient(),
		s.clientManager.GetOperationStrategyClient(),
		s.clientManager.GetStoreClient(),
	)

	// 注册店铺到监控服务
	monitorService.RegisterStore(storeID, apiClient)

	s.sheinMonitors[storeID] = monitorService

	logrus.WithFields(logrus.Fields{
		"store_id":  storeID,
		"tenant_id": tenantID,
	}).Info("注册 SHEIN 店铺到监控调度器")
}

// Start 启动监控调度器
func (s *MonitorScheduler) Start() error {
	logrus.Info("启动产品监控调度器")

	// 启动时立即检查一次
	go s.checkAllStores()

	// 定时检查（使用配置的检查间隔）
	interval := s.monitorConfig.CheckInterval

	// 转换为 cron 表达式（每 N 分钟）
	minutes := int(interval.Minutes())
	cronExpr := fmt.Sprintf("*/%d * * * *", minutes)

	_, err := s.cron.AddFunc(cronExpr, func() {
		s.checkAllStores()
	})
	if err != nil {
		return fmt.Errorf("添加定时任务失败: %w", err)
	}

	s.cron.Start()
	logrus.WithField("interval", interval).Info("监控调度器已启动")
	return nil
}

// Stop 停止监控调度器
func (s *MonitorScheduler) Stop() {
	logrus.Info("停止监控调度器")
	s.cron.Stop()
}

// checkAllStores 检查所有店铺
func (s *MonitorScheduler) checkAllStores() {
	logrus.Info("开始检查所有店铺产品变化")

	for storeID, monitorService := range s.sheinMonitors {
		go func(sid int64, service *shein.SheinProductMonitorService) {
			// 从管理系统获取店铺信息，确保租户 ID 正确
			storeInfo, err := s.clientManager.GetStoreClient().GetStore(sid)
			if err != nil {
				logrus.WithError(err).WithField("store_id", sid).Error("获取店铺信息失败")
				return
			}

			tenantID := storeInfo.TenantID
			if err := service.CheckProductChanges(sid, tenantID); err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"store_id":  sid,
					"tenant_id": tenantID,
				}).Error("店铺产品检查失败")
			}
		}(storeID, monitorService)
	}
}

// CheckNow 立即检查指定店铺
func (s *MonitorScheduler) CheckNow(platform string, storeID int64) error {
	logrus.WithFields(logrus.Fields{
		"platform": platform,
		"store_id": storeID,
	}).Info("手动触发产品检查")

	switch platform {
	case "SHEIN":
		monitorService, ok := s.sheinMonitors[storeID]
		if !ok {
			return fmt.Errorf("SHEIN 店铺 %d 未注册监控", storeID)
		}

		// 从管理系统获取店铺信息，确保租户 ID 正确
		storeInfo, err := s.clientManager.GetStoreClient().GetStore(storeID)
		if err != nil {
			return fmt.Errorf("获取店铺信息失败: %w", err)
		}

		tenantID := storeInfo.TenantID
		return monitorService.CheckProductChanges(storeID, tenantID)

	default:
		return fmt.Errorf("不支持的平台: %s", platform)
	}
}
