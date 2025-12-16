package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"task-processor/common/amazon"
	"task-processor/common/auth"
	"task-processor/common/management"
	"task-processor/common/management/api"
	shops "task-processor/common/shein"
	"task-processor/common/task"
	temuapi "task-processor/common/temu"
	"task-processor/internal/config"
	"task-processor/platforms/common"
	"task-processor/platforms/scheduler"
	"task-processor/platforms/shein"
	"task-processor/platforms/temu"

	"github.com/sirupsen/logrus"
)

// Server represents the web server with all its dependencies
type Server struct {
	cfg                     *config.Config
	clientCredentialsClient *auth.ClientCredentialsAuthClient
	sessionManager          *auth.SessionManager
	managementClient        *management.ClientManager
	temuProcessor           *temu.TemuProcessor
	sheinProcessor          *shein.SheinProcessor
	syncScheduler           *scheduler.SyncScheduler    // 产品同步调度器
	monitorScheduler        *scheduler.MonitorScheduler // 产品监控调度器
	processorCtx            context.Context
	processorCancel         context.CancelFunc
	processorRunning        bool
	logger                  *logrus.Logger
}

// New creates a new server instance
func New(cfg *config.Config, logger *logrus.Logger) *Server {
	return &Server{
		cfg:    cfg,
		logger: logger,
	}
}

// Initialize sets up the server components (deprecated, use InitializeWithClientCredentials)
func (s *Server) Initialize() error {
	s.sessionManager = auth.NewSessionManager()
	return nil
}

// InitializeWithClientCredentials sets up the server with client credentials auth
func (s *Server) InitializeWithClientCredentials(clientCredentialsClient *auth.ClientCredentialsAuthClient) error {
	s.clientCredentialsClient = clientCredentialsClient
	s.sessionManager = auth.NewSessionManager()
	s.autoStartProcessor()
	return nil
}

func (s *Server) autoStartProcessor() {
	s.logger.Info("使用客户端凭证自动启动任务处理器...")

	// 获取访问令牌
	accessToken, err := s.clientCredentialsClient.GetAccessToken()
	if err != nil {
		s.logger.Errorf("获取访问令牌失败: %v", err)
		return
	}

	tenantID := s.clientCredentialsClient.GetTenantID()

	// 初始化任务处理器
	s.initializeTaskProcessor()
	s.setUserTokenToClients(accessToken, tenantID)

	// 启动任务处理器
	go s.startTaskProcessor()

	// 启动产品同步调度器
	go s.startSyncScheduler()

	// 启动产品监控调度器
	go s.startMonitorScheduler()

	s.logger.Info("任务处理器自动启动完成")
}

func (s *Server) initializeTaskProcessor() {
	s.logger.Info("创建任务处理器实例...")

	// 创建共享的 managementClient
	s.logger.Info("创建共享的管理客户端...")
	s.managementClient = management.NewClientManager(&s.cfg.Management)
	// 设置数据新鲜度天数
	s.managementClient.SetDataFreshnessDays(s.cfg.Amazon.DataFreshnessDays)

	// 创建 TEMU 处理器（使用共享的 managementClient）
	// TEMU 处理器内部会创建自己的 WorkerPool
	s.logger.Info("创建 TEMU 任务处理器...")
	s.temuProcessor = temu.NewTemuProcessorWithManagementClient(s.cfg, s.logger, s.managementClient)

	// 获取TEMU处理器的共享Amazon处理器
	sharedAmazonProcessor := s.temuProcessor.GetAmazonProcessor()

	// 创建 SHEIN 处理器（使用共享的 managementClient 和 Amazon处理器）
	s.logger.Info("创建 SHEIN 任务处理器...")
	s.sheinProcessor = shein.NewSheinProcessorWithSharedResources(s.cfg, s.managementClient, sharedAmazonProcessor)

	s.processorCtx, s.processorCancel = context.WithCancel(context.Background())
	s.logger.Info("任务处理器实例创建完成（未启动）")
}

func (s *Server) setUserTokenToClients(accessToken, tenantID string) {
	// 使用共享的 managementClient 设置 token
	if s.managementClient != nil {
		client := s.managementClient.GetClient()
		client.SetUserToken(accessToken, tenantID)
		s.logger.Infof("已设置用户令牌到共享管理客户端 (租户: %s)", tenantID)
	}
}

func (s *Server) startTaskProcessor() {
	if s.processorRunning {
		s.logger.Warn("任务处理器已在运行")
		return
	}

	if s.temuProcessor == nil || s.sheinProcessor == nil {
		s.logger.Info("任务处理器未初始化，正在初始化...")
		s.initializeTaskProcessor()
	}

	if s.temuProcessor == nil || s.sheinProcessor == nil || s.processorCtx == nil {
		s.logger.Error("任务处理器组件初始化失败，无法启动")
		return
	}

	s.logger.Info("启动任务处理器...")

	// 启动 TEMU 任务处理器（内部会启动 WorkerPool）
	s.logger.Info("启动 TEMU 任务处理器...")
	if err := s.temuProcessor.Start(s.processorCtx); err != nil {
		s.logger.Errorf("启动 TEMU 任务处理器失败: %v", err)
		s.rollbackStartup()
		return
	}

	// 启动 SHEIN 任务处理器（内部会启动 WorkerPool）
	s.logger.Info("启动 SHEIN 任务处理器...")
	if err := s.sheinProcessor.Start(s.processorCtx); err != nil {
		s.logger.Errorf("启动 SHEIN 任务处理器失败: %v", err)
		// 回滚：关闭已启动的 TEMU 处理器
		s.temuProcessor.Close()
		s.rollbackStartup()
		return
	}

	// 创建任务提交器
	temuSubmitter := temu.NewTemuTaskSubmitter(s.temuProcessor.GetWorkerPool())
	sheinSubmitter := shein.NewSheinTaskSubmitter(s.sheinProcessor.GetWorkerPool())

	// 创建统一任务获取器
	s.logger.Info("启动统一任务获取器...")
	submitters := map[string]task.TaskSubmitter{
		"TEMU":  temuSubmitter,
		"temu":  temuSubmitter,
		"SHEIN": sheinSubmitter,
		"shein": sheinSubmitter,
	}

	// 使用适配器包装管理客户端
	managementProvider := task.WrapManagementClient(s.managementClient)
	unifiedFetcher := task.NewUnifiedTaskFetcher(s.cfg, managementProvider, submitters)

	// 设置任务完成通知器（让WorkerPool在任务完成后通知UnifiedTaskFetcher）
	if temuPool := s.temuProcessor.GetWorkerPool(); temuPool != nil {
		temuPool.SetCompletionNotifier(unifiedFetcher)
		s.logger.Info("已设置TEMU任务完成通知器")
	}
	if sheinPool := s.sheinProcessor.GetWorkerPool(); sheinPool != nil {
		sheinPool.SetCompletionNotifier(unifiedFetcher)
		s.logger.Info("已设置SHEIN任务完成通知器")
	}

	go unifiedFetcher.Start(s.processorCtx)

	s.processorRunning = true
	s.logger.Info("所有任务处理器启动成功 (TEMU + SHEIN + 统一任务获取器)")
}

// rollbackStartup 回滚启动过程
func (s *Server) rollbackStartup() {
	s.logger.Warn("回滚启动过程...")
	if s.processorCancel != nil {
		s.processorCancel()
	}
	s.processorRunning = false
}

func (s *Server) stopTaskProcessor() {
	if !s.processorRunning {
		s.logger.Info("任务处理器未运行，无需停止")
		return
	}

	s.logger.Info("停止任务处理器...")

	// 停止产品同步调度器
	if s.syncScheduler != nil {
		s.logger.Info("停止产品同步调度器...")
		s.syncScheduler.Stop()
	}

	// 停止产品监控调度器
	if s.monitorScheduler != nil {
		s.logger.Info("停止产品监控调度器...")
		s.monitorScheduler.Stop()
	}

	// 取消 context，通知所有组件停止
	if s.processorCancel != nil {
		s.processorCancel()
	}

	// 等待一小段时间让组件响应取消信号
	time.Sleep(100 * time.Millisecond)

	// 关闭 TEMU 处理器（内部会关闭 WorkerPool）
	if s.temuProcessor != nil {
		s.logger.Info("停止 TEMU 任务处理器...")
		s.temuProcessor.Close()
	}

	// 关闭 SHEIN 处理器（内部会关闭 WorkerPool）
	if s.sheinProcessor != nil {
		s.logger.Info("停止 SHEIN 任务处理器...")
		s.sheinProcessor.Close()
	}

	s.processorRunning = false
	s.logger.Info("所有任务处理器已停止")
}

// ProcessorManager interface implementation
func (s *Server) IsRunning() bool {
	return s.processorRunning
}

func (s *Server) StartProcessor() error {
	s.startTaskProcessor()
	return nil
}

func (s *Server) StopProcessor() error {
	s.stopTaskProcessor()
	return nil
}

func (s *Server) InitializeProcessor() {
	s.initializeTaskProcessor()
}

func (s *Server) SetUserToken(accessToken, tenantID string) {
	s.setUserTokenToClients(accessToken, tenantID)
}

// startSyncScheduler 启动产品同步调度器
func (s *Server) startSyncScheduler() {
	// 检查同步功能是否启用
	if s.cfg.Sync == nil || !s.cfg.Sync.Enabled {
		s.logger.Info("产品同步功能已禁用 (sync.enabled=false)，跳过启动")
		return
	}

	s.logger.Info("启动产品同步调度器...")

	// 创建同步调度器
	s.syncScheduler = scheduler.NewSyncScheduler(s.managementClient)

	// 从管理系统加载店铺配置并注册
	s.loadAndRegisterStores()

	// 启动调度器
	if err := s.syncScheduler.Start(); err != nil {
		s.logger.Errorf("启动产品同步调度器失败: %v", err)
		return
	}

	s.logger.Info("产品同步调度器启动成功")
}

// loadAndRegisterStores 加载并注册店铺到同步调度器
func (s *Server) loadAndRegisterStores() {
	s.logger.Info("开始加载店铺配置...")

	storeClient := s.managementClient.GetStoreClient()
	if storeClient == nil {
		s.logger.Warn("店铺客户端未初始化，跳过店铺注册")
		return
	}

	// 获取租户ID
	tenantID := s.clientCredentialsClient.GetTenantID()
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil {
		s.logger.WithError(err).Error("租户ID转换失败")
		return
	}

	// 获取需要同步的店铺ID列表
	var storeIDs []int64

	// 优先使用 sync.storeIDs，如果未配置则使用 management.storeIDs
	if s.cfg.Sync != nil && len(s.cfg.Sync.StoreIDs) > 0 {
		storeIDs = s.cfg.Sync.StoreIDs
		s.logger.Infof("从 sync.storeIDs 加载 %d 个店铺", len(storeIDs))
	} else if len(s.cfg.Management.StoreIDs) > 0 {
		storeIDs = s.cfg.Management.StoreIDs
		s.logger.Infof("从 management.storeIDs 加载 %d 个店铺", len(storeIDs))
	} else {
		s.logger.Warn("未配置需要同步的店铺ID，产品同步功能将不会运行")
		s.logger.Info("请在配置文件中添加 management.storeIDs 或 sync.storeIDs")
		return
	}

	// 注册店铺
	for _, storeID := range storeIDs {
		s.registerStoreByID(storeID, tenantIDInt)
	}
}

// getStoreInfo 获取并验证店铺信息（公共方法）
func (s *Server) getStoreInfo(storeID int64) (*api.StoreRespDTO, error) {
	storeClient := s.managementClient.GetStoreClient()
	if storeClient == nil {
		return nil, fmt.Errorf("店铺客户端未初始化")
	}

	store, err := storeClient.GetStore(storeID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 检查店铺状态
	if store.Status != 0 {
		return nil, fmt.Errorf("店铺未启用")
	}

	return store, nil
}

// registerStoreByID 根据店铺ID注册店铺到同步调度器
func (s *Server) registerStoreByID(storeID, tenantID int64) {
	store, err := s.getStoreInfo(storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("跳过店铺注册")
		return
	}

	// 根据平台类型注册
	switch store.Platform {
	case "SHEIN":
		s.registerSheinStore(store, tenantID)
	case "TEMU":
		s.registerTemuStore(store, tenantID)
	default:
		s.logger.WithFields(logrus.Fields{
			"store_id": storeID,
			"platform": store.Platform,
		}).Warn("不支持的平台类型")
	}
}

// getSheinAPIClient 获取 SHEIN API 客户端（公共方法）
func (s *Server) getSheinAPIClient(store *api.StoreRespDTO, tenantID int64) (*shops.ShopAPIClient, error) {
	if s.sheinProcessor == nil {
		return nil, fmt.Errorf("SHEIN 处理器未初始化")
	}

	shopClientMgr := s.sheinProcessor.GetShopClientManager()
	if shopClientMgr == nil {
		return nil, fmt.Errorf("SHEIN ClientManager 未初始化")
	}

	apiClient, err := shopClientMgr.GetClient(tenantID, store.ID, store)
	if err != nil {
		// Cookie 不存在，尝试从管理系统 API 获取
		s.logger.WithField("store_id", store.ID).Info("Cookie 不存在，尝试从管理系统 API 获取")

		storeClient := s.managementClient.GetStoreClient()
		cookieJSON, err := storeClient.GetStoreCookie(store.ID)
		if err != nil {
			return nil, fmt.Errorf("从管理系统获取 Cookie 失败: %w", err)
		}

		memoryManager := s.sheinProcessor.GetMemoryManager()
		if memoryManager == nil {
			return nil, fmt.Errorf("MemoryManager 未初始化")
		}

		memoryManager.CookieManager.SetCookie(tenantID, store.ID, cookieJSON)
		s.logger.WithField("store_id", store.ID).Info("✓ Cookie 已从管理系统获取并保存")

		// 重新获取客户端
		apiClient, err = shopClientMgr.GetClient(tenantID, store.ID, store)
		if err != nil {
			return nil, fmt.Errorf("重新获取 SHEIN API 客户端失败: %w", err)
		}
	}

	// 类型断言为具体的 ShopAPIClient 类型
	shopAPIClient, ok := apiClient.(*shops.ShopAPIClient)
	if !ok {
		return nil, fmt.Errorf("API 客户端类型断言失败")
	}

	return shopAPIClient, nil
}

// registerSheinStore 注册 SHEIN 店铺到同步调度器
func (s *Server) registerSheinStore(store *api.StoreRespDTO, tenantID int64) {
	s.logger.WithFields(logrus.Fields{
		"platform":   "SHEIN",
		"store_id":   store.ID,
		"store_name": store.Name,
	}).Info("注册 SHEIN 店铺到同步调度器")

	shopAPIClient, err := s.getSheinAPIClient(store, tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", store.ID).Error("获取 SHEIN API 客户端失败")
		return
	}

	s.syncScheduler.RegisterSheinStore(store.ID, shopAPIClient)

	s.logger.WithFields(logrus.Fields{
		"store_id":  store.ID,
		"tenant_id": tenantID,
	}).Info("✓ SHEIN 店铺注册成功（已加载 Cookie）")
}

// registerTemuStore 注册 TEMU 店铺
func (s *Server) registerTemuStore(store *api.StoreRespDTO, tenantID int64) {
	s.logger.WithFields(logrus.Fields{
		"platform":   "TEMU",
		"store_id":   store.ID,
		"store_name": store.Name,
	}).Info("注册 TEMU 店铺到同步调度器")

	// 创建 TEMU API 客户端
	temuClient := temuapi.NewAPIClient(tenantID, store.ID, s.managementClient)

	// 注册到调度器
	s.syncScheduler.RegisterTemuStore(store.ID, temuClient)

	s.logger.WithFields(logrus.Fields{
		"store_id":  store.ID,
		"tenant_id": tenantID,
	}).Info("✓ TEMU 店铺注册成功")
}

// startMonitorScheduler 启动产品监控调度器
func (s *Server) startMonitorScheduler() {
	// 检查监控功能是否启用
	if s.cfg.Monitor == nil || !s.cfg.Monitor.Enabled {
		s.logger.Info("产品监控功能已禁用 (monitor.enabled=false)，跳过启动")
		return
	}

	s.logger.Info("启动产品监控调度器...")

	// 获取共享的 Amazon 处理器并进行类型断言
	amazonProcessorInterface := s.temuProcessor.GetAmazonProcessor()
	amazonProcessor, ok := amazonProcessorInterface.(*amazon.AmazonProcessor)
	if !ok {
		s.logger.Error("Amazon 处理器类型断言失败，无法启动监控调度器")
		return
	}

	// 创建监控配置（从配置文件转换为 common.MonitorConfig）
	monitorConfig := &common.MonitorConfig{
		CheckInterval:        time.Duration(s.cfg.Monitor.CheckInterval) * time.Minute,
		BatchSize:            s.cfg.Monitor.BatchSize,
		EnablePriceAlert:     s.cfg.Monitor.EnablePriceAlert,
		EnableStockAlert:     s.cfg.Monitor.EnableStockAlert,
		PriceChangeThreshold: s.cfg.Monitor.PriceChangeThreshold,
		StockChangeThreshold: s.cfg.Monitor.StockChangeThreshold,
	}

	// 创建监控事件处理器
	eventHandler := common.NewLogMonitorEventHandler()

	// 创建监控调度器
	s.monitorScheduler = scheduler.NewMonitorScheduler(
		s.managementClient,
		amazonProcessor,
		monitorConfig,
		eventHandler,
	)

	// 从管理系统加载店铺配置并注册到监控调度器
	s.loadAndRegisterMonitorStores()

	// 启动调度器
	if err := s.monitorScheduler.Start(); err != nil {
		s.logger.Errorf("启动产品监控调度器失败: %v", err)
		return
	}

	s.logger.Info("产品监控调度器启动成功")
}

// loadAndRegisterMonitorStores 加载并注册店铺到监控调度器
func (s *Server) loadAndRegisterMonitorStores() {
	s.logger.Info("开始加载店铺配置到监控调度器...")

	storeClient := s.managementClient.GetStoreClient()
	if storeClient == nil {
		s.logger.Warn("店铺客户端未初始化，跳过店铺注册")
		return
	}

	// 获取租户ID
	tenantID := s.clientCredentialsClient.GetTenantID()
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil {
		s.logger.WithError(err).Error("租户ID转换失败")
		return
	}

	// 获取需要监控的店铺ID列表
	var storeIDs []int64

	// 优先使用 monitor.storeIDs，如果未配置则使用 sync.storeIDs 或 management.storeIDs
	if s.cfg.Monitor != nil && len(s.cfg.Monitor.StoreIDs) > 0 {
		storeIDs = s.cfg.Monitor.StoreIDs
		s.logger.Infof("从 monitor.storeIDs 加载 %d 个店铺", len(storeIDs))
	} else if s.cfg.Sync != nil && len(s.cfg.Sync.StoreIDs) > 0 {
		storeIDs = s.cfg.Sync.StoreIDs
		s.logger.Infof("从 sync.storeIDs 加载 %d 个店铺", len(storeIDs))
	} else if len(s.cfg.Management.StoreIDs) > 0 {
		storeIDs = s.cfg.Management.StoreIDs
		s.logger.Infof("从 management.storeIDs 加载 %d 个店铺", len(storeIDs))
	} else {
		s.logger.Warn("未配置需要监控的店铺ID，产品监控功能将不会运行")
		return
	}

	// 注册店铺到监控调度器
	for _, storeID := range storeIDs {
		s.registerMonitorStoreByID(storeID, tenantIDInt)
	}
}

// registerMonitorStoreByID 根据店铺ID注册店铺到监控调度器
func (s *Server) registerMonitorStoreByID(storeID, tenantID int64) {
	store, err := s.getStoreInfo(storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("跳过监控注册")
		return
	}

	// 目前只支持 SHEIN 平台的监控
	if store.Platform != "SHEIN" {
		s.logger.WithFields(logrus.Fields{
			"store_id": storeID,
			"platform": store.Platform,
		}).Debug("暂不支持该平台的监控")
		return
	}

	s.registerSheinMonitorStore(store, tenantID)
}

// registerSheinMonitorStore 注册 SHEIN 店铺到监控调度器
func (s *Server) registerSheinMonitorStore(store *api.StoreRespDTO, tenantID int64) {
	s.logger.WithFields(logrus.Fields{
		"platform":   "SHEIN",
		"store_id":   store.ID,
		"store_name": store.Name,
	}).Info("注册 SHEIN 店铺到监控调度器")

	shopAPIClient, err := s.getSheinAPIClient(store, tenantID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", store.ID).Error("获取 SHEIN API 客户端失败")
		return
	}

	s.monitorScheduler.RegisterSheinStore(store.ID, shopAPIClient)

	s.logger.WithFields(logrus.Fields{
		"store_id":  store.ID,
		"tenant_id": tenantID,
	}).Info("✓ SHEIN 店铺监控注册成功")
}

// GetSyncScheduler 获取同步调度器（用于手动触发同步）
func (s *Server) GetSyncScheduler() *scheduler.SyncScheduler {
	return s.syncScheduler
}

// GetMonitorScheduler 获取监控调度器（用于手动触发监控）
func (s *Server) GetMonitorScheduler() *scheduler.MonitorScheduler {
	return s.monitorScheduler
}

// Getter methods for exposing components to handlers
func (s *Server) GetSessionManager() *auth.SessionManager {
	return s.sessionManager
}

func (s *Server) GetClientCredentialsClient() *auth.ClientCredentialsAuthClient {
	return s.clientCredentialsClient
}
