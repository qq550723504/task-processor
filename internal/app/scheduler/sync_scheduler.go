package scheduler

import (
	"fmt"
	shops "task-processor/internal/common/shein"
	temuapi "task-processor/internal/common/temu"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// SyncScheduler 多平台同步调度器
type SyncScheduler struct {
	cron            *cron.Cron
	sheinServices   map[int64]*shein.SyncService   // storeID -> SyncService
	sheinAPIClients map[int64]*shops.ShopAPIClient // storeID -> APIClient
	temuAPIClients  map[int64]*temuapi.APIClient   // storeID -> APIClient
	temuServices    map[int64]*temu.SyncService    // storeID -> SyncService
	clientManager   *management.ClientManager
}

// NewSyncScheduler 创建同步调度器
func NewSyncScheduler(clientManager *management.ClientManager) *SyncScheduler {
	return &SyncScheduler{
		cron:            cron.New(),
		sheinServices:   make(map[int64]*shein.SyncService),
		sheinAPIClients: make(map[int64]*shops.ShopAPIClient),
		temuAPIClients:  make(map[int64]*temuapi.APIClient),
		temuServices:    make(map[int64]*temu.SyncService),
		clientManager:   clientManager,
	}
}

// RegisterSheinStore 注册 SHEIN 店铺
func (s *SyncScheduler) RegisterSheinStore(storeID int64, apiClient *shops.ShopAPIClient) {
	s.sheinAPIClients[storeID] = apiClient
	tenantID := apiClient.GetTenantID()

	// 使用公共方法创建同步服务
	syncService := createSheinSyncService(s.clientManager)
	s.sheinServices[storeID] = syncService

	logrus.WithFields(logrus.Fields{
		"store_id":  storeID,
		"tenant_id": tenantID,
	}).Info("注册 SHEIN 店铺到同步调度器")
}

// RegisterTemuStore 注册 TEMU 店铺
func (s *SyncScheduler) RegisterTemuStore(storeID int64, temuAPIClient interface{}) {
	apiClient := temuAPIClient.(*temuapi.APIClient)
	s.temuAPIClients[storeID] = apiClient

	// 获取租户ID
	tenantID := apiClient.GetTenantID()

	// 创建 repository 工厂函数
	repositoryFactory := func(storeID, tenantID int64) api.ProductDataAPI {
		return s.clientManager.GetProductDataClientWithTenant(storeID, tenantID)
	}

	// 为每个店铺创建独立的同步服务
	syncService := temu.NewSyncService(repositoryFactory, apiClient)

	// 设置映射客户端，用于查询 ASIN
	syncService.SetMappingClient(s.clientManager.GetProductImportMappingClient())

	s.temuServices[storeID] = syncService

	logrus.WithFields(logrus.Fields{
		"store_id":  storeID,
		"tenant_id": tenantID,
	}).Info("注册 TEMU 店铺")
}

// Start 启动调度器
func (s *SyncScheduler) Start() error {
	logrus.Info("启动多平台同步调度器")

	// 启动时立即同步一次所有店铺
	go s.syncAllStores("initial")

	// 高频同步：每 30 分钟同步一次（最近 7 天的产品）
	_, err := s.cron.AddFunc("*/30 * * * *", func() {
		s.syncAllStores("high")
	})
	if err != nil {
		return err
	}

	// 中频同步：每 2 小时同步一次（7-30 天的产品）
	_, err = s.cron.AddFunc("0 */2 * * *", func() {
		s.syncAllStores("medium")
	})
	if err != nil {
		return err
	}

	// 低频同步：每天凌晨 2 点同步一次（30 天以上的产品）
	_, err = s.cron.AddFunc("0 2 * * *", func() {
		s.syncAllStores("low")
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	logrus.Info("同步调度器已启动")
	return nil
}

// Stop 停止调度器
func (s *SyncScheduler) Stop() {
	logrus.Info("停止同步调度器")
	s.cron.Stop()
}

// syncAllStores 同步所有店铺
func (s *SyncScheduler) syncAllStores(frequency string) {
	logrus.WithField("frequency", frequency).Info("开始同步所有店铺")

	// 同步所有 SHEIN 店铺
	for storeID, apiClient := range s.sheinAPIClients {
		go func(sid int64, client *shops.ShopAPIClient) {
			if err := s.syncSheinStore(sid, client); err != nil {
				logrus.WithError(err).WithField("store_id", sid).Error("SHEIN 店铺同步失败")
			}
		}(storeID, apiClient)
	}

	// 同步所有 TEMU 店铺
	for storeID, syncService := range s.temuServices {
		go func(sid int64, service *temu.SyncService) {
			if err := s.syncTemuStore(sid, service); err != nil {
				logrus.WithError(err).WithField("store_id", sid).Error("TEMU 店铺同步失败")
			}
		}(storeID, syncService)
	}
}

// syncSheinStore 同步单个 SHEIN 店铺
func (s *SyncScheduler) syncSheinStore(storeID int64, apiClient *shops.ShopAPIClient) error {
	start := time.Now()

	// 获取同步服务
	syncService, ok := s.sheinServices[storeID]
	if !ok {
		return fmt.Errorf("SHEIN 店铺 %d 未注册", storeID)
	}

	// 从管理系统 API 获取店铺信息
	storeInfo, err := s.clientManager.GetStoreClient().GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	tenantID := storeInfo.TenantID

	logrus.WithFields(logrus.Fields{
		"store_id":  storeID,
		"tenant_id": tenantID,
		"shop_type": storeInfo.ShopType,
	}).Info("开始同步 SHEIN 店铺")

	count, err := syncService.SyncProducts(apiClient, tenantID, storeID, storeInfo.ShopType)
	if err != nil {
		return err
	}

	duration := time.Since(start)
	logrus.WithFields(logrus.Fields{
		"store_id": storeID,
		"count":    count,
		"duration": duration,
	}).Info("SHEIN 店铺同步完成")

	return nil
}

// syncTemuStore 同步单个 TEMU 店铺
func (s *SyncScheduler) syncTemuStore(storeID int64, syncService *temu.SyncService) error {
	start := time.Now()

	// 从管理系统 API 获取店铺信息
	storeInfo, err := s.clientManager.GetStoreClient().GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	tenantID := storeInfo.TenantID

	logrus.WithFields(logrus.Fields{
		"store_id":  storeID,
		"tenant_id": tenantID,
	}).Info("开始同步 TEMU 店铺")

	count, err := syncService.SyncProducts(tenantID, storeID)
	if err != nil {
		return err
	}

	duration := time.Since(start)
	logrus.WithFields(logrus.Fields{
		"store_id": storeID,
		"count":    count,
		"duration": duration,
	}).Info("TEMU 店铺同步完成")

	return nil
}

// SyncNow 立即同步指定店铺
func (s *SyncScheduler) SyncNow(platform string, storeID int64) error {
	logrus.WithFields(logrus.Fields{
		"platform": platform,
		"store_id": storeID,
	}).Info("手动触发同步")

	switch platform {
	case "SHEIN":
		apiClient, ok := s.sheinAPIClients[storeID]
		if !ok {
			err := fmt.Errorf("SHEIN 店铺 %d 未注册", storeID)
			logrus.WithError(err).Error("店铺未注册")
			return err
		}
		return s.syncSheinStore(storeID, apiClient)

	case "TEMU":
		syncService, ok := s.temuServices[storeID]
		if !ok {
			err := fmt.Errorf("TEMU 店铺 %d 未注册", storeID)
			logrus.WithError(err).Error("店铺未注册")
			return err
		}
		return s.syncTemuStore(storeID, syncService)

	default:
		err := fmt.Errorf("不支持的平台: %s", platform)
		logrus.WithError(err).Error("平台不支持")
		return err
	}
}
