// Package consumer 提供RabbitMQ服务管理器
package consumer

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	apptask "task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

const sheinBucketQueueCount = 8

// RabbitMQService RabbitMQ服务管理器
// 结构上分为三类职责：
// 1）纯基础设施职责：连接管理、客户端、消费者、队列初始化、处理器注册。
// 2）业务辅助依赖（ResultReporter/StoreAPI/Deduplicator）：仅通过 TaskProcessorRegistry 和预加载逻辑为处理器提供上下文。
// 3）生命周期和状态管理：启动/停止、重连回调、状态统计。
type RabbitMQService struct {
	config            *config.RabbitMQConfig
	connManager       *rabbitmq.ConnectionManager
	client            *rabbitmq.Client
	consumer          *rabbitmq.MessageConsumer
	initializer       *rabbitmq.QueueInitializer
	processorRegistry *TaskProcessorRegistry
	logger            *logrus.Logger

	// 新增组件
	resultReporter          *ResultReporter
	storeAPI                api.StoreAPI
	ownedStores             []int64
	ownedBuckets            []int
	useStoreQueues          bool
	storeAssignmentProvider StoreAssignmentProvider
	deduplicator            *apptask.DeduplicationManager

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态管理
	started        bool
	consumerActive bool
	mutex          sync.RWMutex
}

type consumerLifecycleState struct {
	started        bool
	consumerActive bool
	ctx            context.Context
}

type serviceStopState struct {
	started      bool
	consumer     *rabbitmq.MessageConsumer
	deduplicator *apptask.DeduplicationManager
	cancel       context.CancelFunc
	connManager  *rabbitmq.ConnectionManager
	provider     StoreAssignmentProvider
}

const consumerGuardInterval = 5 * time.Second

type consumerReconcileAction string

const (
	consumerActionNone    consumerReconcileAction = "none"
	consumerActionPause   consumerReconcileAction = "pause"
	consumerActionResume  consumerReconcileAction = "resume"
	consumerActionRestart consumerReconcileAction = "restart"
)

// NewRabbitMQService 创建RabbitMQ服务
func NewRabbitMQService(cfg *config.RabbitMQConfig, logger *logrus.Logger) *RabbitMQService {
	applyRabbitMQServiceDefaults(cfg)

	connManager := newRabbitMQConnectionManager(cfg, logger)
	client := rabbitmq.NewClient(connManager, logger)
	consumer := newRabbitMQConsumer(client, cfg, logger)
	initializer := rabbitmq.NewQueueInitializer(client, logger)
	processorRegistry := NewTaskProcessorRegistry(nil, nil, nil, false, nil, logger)

	return &RabbitMQService{
		config:            cfg,
		connManager:       connManager,
		client:            client,
		consumer:          consumer,
		initializer:       initializer,
		processorRegistry: processorRegistry,
		logger:            logger,
		ownedStores:       append([]int64(nil), cfg.Node.OwnedStores...),
		ownedBuckets:      normalizeOwnedBuckets(cfg.Node.OwnedBuckets),
		useStoreQueues:    cfg.Node.UseStoreQueues,
	}
}

// RegisterProcessor 注册任务处理器
func (s *RabbitMQService) RegisterProcessor(platform string, processor worker.Processor) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.started {
		return fmt.Errorf("服务已启动，无法注册新的处理器")
	}

	// 注册到处理器注册表
	s.processorRegistry.RegisterProcessor(platform, processor)

	s.logger.Infof("注册RabbitMQ任务处理器: 平台=%s", platform)
	return nil
}

// SetComponents 设置服务组件（ResultReporter、StoreAPI、Deduplicator）
func (s *RabbitMQService) SetComponents(
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	deduplicator *apptask.DeduplicationManager,
) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.applyComponentDependencies(resultReporter, storeAPI, ownedStores, deduplicator)
	s.syncProcessorRegistryComponents()

	s.logger.Info("设置服务组件完成")
}

// SetStoreAssignmentProvider enables dynamic owned-store refresh for dedicated queue nodes.
func (s *RabbitMQService) SetStoreAssignmentProvider(provider StoreAssignmentProvider) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.storeAssignmentProvider = provider
	s.useStoreQueues = s.config.Node.UseStoreQueues || provider != nil
	s.syncProcessorRegistryComponents()
}

// SetQueueConfigs 设置队列配置
func (s *RabbitMQService) SetQueueConfigs(configs []config.QueueConfig) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	configs = s.filterQueueConfigsByRole(configs)

	// 转换为内部类型
	queueConfigs := make([]rabbitmq.ConsumerQueueConfig, len(configs))
	for i, cfg := range configs {
		queueConfigs[i] = rabbitmq.ConsumerQueueConfig{
			Name:     cfg.Name,
			Priority: cfg.Priority,
			Prefetch: cfg.Prefetch,
		}
	}

	s.consumer.SetQueueConfigs(queueConfigs)
	s.logger.Infof("设置队列配置: %d个队列, role=%s", len(configs), s.config.Node.NormalizedRole())
}

// Start 启动RabbitMQ服务
func (s *RabbitMQService) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.started {
		s.mutex.Unlock()
		return fmt.Errorf("RabbitMQ服务已启动")
	}
	s.logger.Info("启动RabbitMQ服务...")
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mutex.Unlock()

	if err := s.startInfrastructure(); err != nil {
		return err
	}
	if err := s.initializeQueueTopology(); err != nil {
		return err
	}
	if err := s.startConsumers(); err != nil {
		return err
	}

	s.mutex.Lock()
	s.started = true
	s.consumerActive = true
	s.mutex.Unlock()
	s.startConsumerGuard()
	s.startStoreAssignmentSync()
	s.logger.Info("RabbitMQ服务启动完成")
	return nil
}

func (s *RabbitMQService) startInfrastructure() error {
	// 建立连接。启动阶段不因依赖瞬时不可用直接退出，而是持续重试直到成功或上下文取消。
	if err := s.connectWithRetry(s.ctx); err != nil {
		return fmt.Errorf("建立RabbitMQ连接失败: %w", err)
	}

	// 注册重连回调（重连后自动重启消费者）
	s.connManager.RegisterReconnectCallback(func() error {
		return s.handleReconnect()
	})
	return nil
}

func (s *RabbitMQService) handleReconnect() error {
	s.logger.Info("连接恢复，重新初始化队列和重启消费者...")

	if err := s.initializer.InitializeAll(); err != nil {
		s.logger.Errorf("重连后初始化队列失败: %v", err)
		return err
	}
	if err := s.consumer.Restart(); err != nil {
		s.logger.Errorf("重连后重启消费者失败: %v", err)
		return err
	}

	s.logger.Info("✅ 重连后恢复完成")
	return nil
}

func (s *RabbitMQService) initializeQueueTopology() error {
	if err := s.initializer.InitializeAll(); err != nil {
		return fmt.Errorf("初始化RabbitMQ队列失败: %w", err)
	}

	// 在首次注册处理器前，先同步一次动态店铺归属，避免新分片启动时短暂回退到共享队列。
	s.syncInitialStoreAssignments(s.ctx)

	if err := s.initializeTaskQueues(); err != nil {
		return err
	}
	if err := s.initializeCrawlerQueues(); err != nil {
		return err
	}
	if s.usesDedicatedStoreQueues() {
		s.preloadOwnedStoreConfigs(s.ownedStores)
	}

	s.registerMessageHandlers()
	return nil
}

func (s *RabbitMQService) initializeTaskQueues() error {
	if !s.config.Node.HandlesTaskWork() {
		return nil
	}

	platforms := s.getRegisteredPlatforms()
	if !s.usesDedicatedStoreQueues() && len(platforms) > 0 {
		if err := s.initializer.InitializePlatformQueues(platforms); err != nil {
			return fmt.Errorf("初始化平台共享队列失败: %w", err)
		}
		s.logger.Infof("平台共享队列初始化完成: platforms=%v", platforms)
		return nil
	}

	if s.usesDedicatedStoreQueues() && len(s.ownedStores) > 0 {
		for _, platform := range platforms {
			if err := s.initializer.InitializeStoreQueues(platform, s.ownedStores); err != nil {
				return fmt.Errorf("初始化平台 %s 店铺队列失败: %w", platform, err)
			}
		}
		s.logger.Infof("店铺专属队列初始化完成: platforms=%v, stores=%v", platforms, s.ownedStores)
	}
	return nil
}

func (s *RabbitMQService) initializeCrawlerQueues() error {
	if !s.config.Node.HandlesCrawlerWork() || len(s.config.Node.Regions) == 0 {
		return nil
	}
	if err := s.initializer.InitializeRegionCrawlerQueues(s.config.Node.Regions); err != nil {
		return fmt.Errorf("初始化 region 爬虫队列失败: %w", err)
	}
	return nil
}

func (s *RabbitMQService) startConsumers() error {
	if err := s.consumer.Start(s.ctx); err != nil {
		return fmt.Errorf("启动消息消费者失败: %w", err)
	}
	return nil
}

func (s *RabbitMQService) connectWithRetry(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		err := s.connManager.Connect(ctx)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		delay := startupRetryDelay(attempt, s.config.ReconnectInterval)

		s.logger.Warnf("RabbitMQ 初始连接失败，第 %d 次重试前等待 %v: %v", attempt+1, delay, err)
		if err := waitForRetry(ctx, delay); err != nil {
			return err
		}
	}
}

func startupRetryDelay(attempt int, base time.Duration) time.Duration {
	if base <= 0 {
		base = 5 * time.Second
	}

	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// registerMessageHandlers 注册消息处理器
func (s *RabbitMQService) registerMessageHandlers() {
	s.consumer.ReplaceHandlers(s.buildQueueHandlers())
}

func (s *RabbitMQService) buildQueueHandlers() map[string]rabbitmq.MessageHandler {
	return newQueueHandlerBuilder(s).build()
}

func (s *RabbitMQService) sharedBucketsForPlatform(platform string) []int {
	if !strings.EqualFold(platform, "shein") {
		return nil
	}
	if len(s.ownedBuckets) > 0 {
		return append([]int(nil), s.ownedBuckets...)
	}

	buckets := make([]int, 0, sheinBucketQueueCount)
	for bucket := 0; bucket < sheinBucketQueueCount; bucket++ {
		buckets = append(buckets, bucket)
	}
	return buckets
}

func (s *RabbitMQService) usesDedicatedStoreQueues() bool {
	return s.useStoreQueues || s.storeAssignmentProvider != nil
}

func normalizeOwnedBuckets(buckets []int) []int {
	if len(buckets) == 0 {
		return nil
	}

	seen := make(map[int]struct{}, len(buckets))
	normalized := make([]int, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket < 0 || bucket >= sheinBucketQueueCount {
			continue
		}
		if _, exists := seen[bucket]; exists {
			continue
		}
		seen[bucket] = struct{}{}
		normalized = append(normalized, bucket)
	}
	slices.Sort(normalized)
	return normalized
}

func (s *RabbitMQService) filterQueueConfigsByRole(configs []config.QueueConfig) []config.QueueConfig {
	if len(configs) == 0 {
		return configs
	}

	filtered := make([]config.QueueConfig, 0, len(configs))
	for _, cfg := range configs {
		if s.shouldUseQueueConfig(cfg.Name) {
			filtered = append(filtered, cfg)
		}
	}
	return filtered
}

func (s *RabbitMQService) shouldUseQueueConfig(name string) bool {
	isCrawlerQueue := strings.Contains(strings.ToLower(name), ".crawler")
	if isCrawlerQueue {
		return s.config.Node.HandlesCrawlerWork()
	}
	return s.config.Node.HandlesTaskWork()
}

// getRegisteredPlatforms 获取已注册的非爬虫平台列表
func (s *RabbitMQService) getRegisteredPlatforms() []string {
	handlers := s.processorRegistry.GetAllHandlers()
	platforms := make([]string, 0, len(handlers))
	for platform := range handlers {
		if platform != "amazon.crawler" && platform != "1688.crawler" {
			platforms = append(platforms, platform)
		}
	}
	return platforms
}

func (s *RabbitMQService) startStoreAssignmentSync() {
	newStoreAssignmentSyncCoordinator(s).start()
}

func (s *RabbitMQService) syncInitialStoreAssignments(ctx context.Context) {
	newStoreAssignmentSyncCoordinator(s).syncInitial(ctx)
}

func (s *RabbitMQService) startConsumerGuard() {
	newConsumerGuardCoordinator(s).start()
}

func decideConsumerAction(started, connected, consumerActive, consumersHealthy bool) consumerReconcileAction {
	if !started {
		return consumerActionNone
	}
	if !connected {
		if consumerActive {
			return consumerActionPause
		}
		return consumerActionNone
	}
	if !consumerActive {
		return consumerActionResume
	}
	if !consumersHealthy {
		return consumerActionRestart
	}
	return consumerActionNone
}

func (s *RabbitMQService) consumerLifecycleStateSnapshot() consumerLifecycleState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return consumerLifecycleState{
		started:        s.started,
		consumerActive: s.consumerActive,
		ctx:            s.ctx,
	}
}

func (s *RabbitMQService) setConsumerActive(active bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.consumerActive = active
}

func (s *RabbitMQService) stopStateSnapshot() serviceStopState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return serviceStopState{
		started:      s.started,
		consumer:     s.consumer,
		deduplicator: s.deduplicator,
		cancel:       s.cancel,
		connManager:  s.connManager,
		provider:     s.storeAssignmentProvider,
	}
}

func (s *RabbitMQService) markStopped() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.started = false
	s.consumerActive = false
}

func (s *RabbitMQService) pauseConsumers(ctx context.Context, reason string) error {
	state := s.consumerLifecycleStateSnapshot()
	if !state.started || !state.consumerActive {
		return nil
	}

	s.logger.WithField("reason", reason).Warn("暂停 RabbitMQ 消费者")
	if err := s.consumer.Stop(ctx); err != nil {
		return err
	}

	s.setConsumerActive(false)
	return nil
}

func (s *RabbitMQService) resumeConsumers(reason string) error {
	state := s.consumerLifecycleStateSnapshot()
	if !state.started || state.consumerActive || state.ctx == nil {
		return nil
	}

	s.logger.WithField("reason", reason).Info("恢复 RabbitMQ 消费者")
	if err := s.consumer.Start(state.ctx); err != nil {
		return err
	}

	s.setConsumerActive(true)
	return nil
}

func (s *RabbitMQService) restartConsumers(reason string) error {
	state := s.consumerLifecycleStateSnapshot()
	if !state.started || !state.consumerActive {
		return nil
	}

	s.logger.WithField("reason", reason).Warn("重启 RabbitMQ 消费者")
	if err := s.consumer.Restart(); err != nil {
		return err
	}
	return nil
}

func (s *RabbitMQService) preloadOwnedStoreConfigs(storeIDs []int64) {
	if s.storeAPI == nil || len(storeIDs) == 0 {
		return
	}

	s.logger.Infof("开始预加载 %d 个店铺配置...", len(storeIDs))
	for _, storeID := range storeIDs {
		_, err := s.storeAPI.GetStore(storeID)
		if err != nil {
			s.logger.Warnf("预加载店铺 %d 配置失败: %v", storeID, err)
			continue
		}
		s.logger.Infof("✅ 预加载店铺 %d 配置成功", storeID)
	}
	s.logger.Info("店铺配置预加载完成")
}

func (s *RabbitMQService) waitForBackgroundWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop 停止RabbitMQ服务
func (s *RabbitMQService) Stop(ctx context.Context) error {
	state := s.stopStateSnapshot()
	if !state.started {
		s.logger.Info("RabbitMQ服务未启动，无需停止")
		return nil
	}

	s.logger.Info("停止RabbitMQ服务...")
	s.markStopped()

	// 1. 停止消费者
	if err := state.consumer.Stop(ctx); err != nil {
		s.logger.Errorf("停止消费者失败: %v", err)
		return err
	}

	// 2. 停止去重器
	if state.deduplicator != nil {
		state.deduplicator.Stop()
	}

	// 3. 取消上下文
	if state.cancel != nil {
		state.cancel()
	}

	// 4. 等待后台协程退出，避免 guard / assignment sync 在资源关闭后继续工作
	if err := s.waitForBackgroundWorkers(ctx); err != nil {
		s.logger.Errorf("等待RabbitMQ后台协程退出失败: %v", err)
		return err
	}

	// 5. 关闭连接
	if err := state.connManager.Close(); err != nil {
		s.logger.Errorf("关闭RabbitMQ连接失败: %v", err)
		return err
	}

	if state.provider != nil {
		if err := state.provider.Close(); err != nil {
			s.logger.Errorf("关闭动态店铺归属提供器失败: %v", err)
			return err
		}
	}

	s.logger.Info("RabbitMQ服务停止完成")
	return nil
}

// IsConnected 检查连接状态
func (s *RabbitMQService) IsConnected() bool {
	return s.connManager.IsConnected()
}

// HasHealthyRequiredConsumers 检查当前节点所需消费者是否都健康。
func (s *RabbitMQService) HasHealthyRequiredConsumers() bool {
	if s.consumer == nil {
		return true
	}
	return s.consumer.HasHealthyRequiredConsumers()
}

// GetUnhealthyRequiredQueues 返回不健康的必需队列。
func (s *RabbitMQService) GetUnhealthyRequiredQueues() []string {
	if s.consumer == nil {
		return nil
	}
	return s.consumer.GetUnhealthyRequiredQueues()
}

// GetStats 获取服务统计信息
func (s *RabbitMQService) GetStats() map[string]any {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := make(map[string]any)
	stats["started"] = s.started
	stats["connected"] = s.IsConnected()
	stats["required_consumers_healthy"] = s.HasHealthyRequiredConsumers()
	stats["unhealthy_required_queues"] = s.GetUnhealthyRequiredQueues()
	stats["use_store_queues"] = s.useStoreQueues
	stats["owned_stores"] = append([]int64(nil), s.ownedStores...)
	stats["owned_buckets"] = append([]int(nil), s.ownedBuckets...)

	// 处理器统计
	stats["processors"] = s.processorRegistry.GetStats()

	// 消费者统计
	if s.consumer != nil {
		stats["consumer"] = s.consumer.GetQueueStats()
	}

	return stats
}

// GetClient 获取RabbitMQ客户端
func (s *RabbitMQService) GetClient() *rabbitmq.Client {
	return s.client
}

// GetConsumer 获取消息消费者
func (s *RabbitMQService) GetConsumer() *rabbitmq.MessageConsumer {
	return s.consumer
}
