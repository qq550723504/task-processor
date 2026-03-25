// Package consumer 提供RabbitMQ服务管理器
package consumer

import (
	"context"
	"fmt"
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
	resultReporter *ResultReporter
	storeAPI       api.StoreAPI
	ownedStores    []int64
	deduplicator   *apptask.DeduplicationManager

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc

	// 状态管理
	started bool
	mutex   sync.RWMutex
}

// NewRabbitMQService 创建RabbitMQ服务
func NewRabbitMQService(cfg *config.RabbitMQConfig, logger *logrus.Logger) *RabbitMQService {
	// 设置默认值
	if cfg.ReconnectInterval == 0 {
		cfg.ReconnectInterval = 5 * time.Second
	}
	if cfg.MaxReconnectTries == 0 {
		cfg.MaxReconnectTries = 10
	}
	if cfg.Consumer.PrefetchCount == 0 {
		cfg.Consumer.PrefetchCount = 1
	}
	if cfg.Consumer.RetryDelay == 0 {
		cfg.Consumer.RetryDelay = 5 * time.Second
	}
	if cfg.Consumer.MaxRetries == 0 {
		cfg.Consumer.MaxRetries = 3
	}

	// 创建连接管理器
	connConfig := rabbitmq.ConnectionConfig{
		URL:               cfg.URL,
		ReconnectInterval: cfg.ReconnectInterval,
		MaxReconnectTries: cfg.MaxReconnectTries,
	}
	connManager := rabbitmq.NewConnectionManager(connConfig, logger)

	// 创建客户端
	client := rabbitmq.NewClient(connManager, logger)

	// 转换消费者配置
	consumerConfig := rabbitmq.ConsumerConfig{
		PrefetchCount: cfg.Consumer.PrefetchCount,
		PrefetchSize:  cfg.Consumer.PrefetchSize,
		RetryDelay:    cfg.Consumer.RetryDelay,
		MaxRetries:    cfg.Consumer.MaxRetries,
	}
	consumer := rabbitmq.NewMessageConsumer(client, consumerConfig, logger)

	// 创建初始化器
	initializer := rabbitmq.NewQueueInitializer(client, logger)

	// 创建增强的处理器注册表（暂时传nil，后续通过SetComponents设置）
	processorRegistry := NewTaskProcessorRegistry(nil, nil, nil, nil, logger)

	return &RabbitMQService{
		config:            cfg,
		connManager:       connManager,
		client:            client,
		consumer:          consumer,
		initializer:       initializer,
		processorRegistry: processorRegistry,
		logger:            logger,
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

	// 只更新非 nil 的字段，避免覆盖已有组件
	if resultReporter != nil {
		s.resultReporter = resultReporter
	}
	if storeAPI != nil {
		s.storeAPI = storeAPI
	}
	if ownedStores != nil {
		s.ownedStores = ownedStores
	}
	if deduplicator != nil {
		s.deduplicator = deduplicator
	}

	// 原地更新注册表依赖，避免重建和迁移注册表对象。
	s.processorRegistry.UpdateComponents(
		s.resultReporter,
		s.storeAPI,
		s.ownedStores,
		s.deduplicator,
	)

	s.logger.Info("设置服务组件完成")
}

// SetQueueConfigs 设置队列配置
func (s *RabbitMQService) SetQueueConfigs(configs []config.QueueConfig) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	s.logger.Infof("设置队列配置: %d个队列", len(configs))
}

// Start 启动RabbitMQ服务
func (s *RabbitMQService) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.started {
		return fmt.Errorf("RabbitMQ服务已启动")
	}

	s.logger.Info("启动RabbitMQ服务...")

	s.ctx, s.cancel = context.WithCancel(ctx)

	// 1. 建立连接
	if err := s.connManager.Connect(s.ctx); err != nil {
		return fmt.Errorf("建立RabbitMQ连接失败: %w", err)
	}

	// 2. 注册重连回调（重连后自动重启消费者）
	s.connManager.RegisterReconnectCallback(func() error {
		s.logger.Info("连接恢复，重新初始化队列和重启消费者...")

		// 重新初始化队列
		if err := s.initializer.InitializeAll(); err != nil {
			s.logger.Errorf("重连后初始化队列失败: %v", err)
			return err
		}

		// 重启消费者
		if err := s.consumer.Restart(); err != nil {
			s.logger.Errorf("重连后重启消费者失败: %v", err)
			return err
		}

		s.logger.Info("✅ 重连后恢复完成")
		return nil
	})

	// 3. 初始化队列和交换机
	if err := s.initializer.InitializeAll(); err != nil {
		return fmt.Errorf("初始化RabbitMQ队列失败: %w", err)
	}

	// 4. 初始化店铺专属队列（如果配置了 ownedStores）
	if len(s.ownedStores) > 0 {
		platforms := s.getRegisteredPlatforms()
		for _, platform := range platforms {
			if err := s.initializer.InitializeStoreQueues(platform, s.ownedStores); err != nil {
				return fmt.Errorf("初始化平台 %s 店铺队列失败: %w", platform, err)
			}
		}
		s.logger.Infof("店铺专属队列初始化完成: platforms=%v, stores=%v", platforms, s.ownedStores)
	}

	// 4.1 初始化 region 爬虫队列（如果配置了 regions）
	if len(s.config.Node.Regions) > 0 {
		if err := s.initializer.InitializeRegionCrawlerQueues(s.config.Node.Regions); err != nil {
			return fmt.Errorf("初始化 region 爬虫队列失败: %w", err)
		}
	}

	// 4. 预加载店铺配置（如果启用）
	if s.storeAPI != nil && len(s.ownedStores) > 0 {
		s.logger.Infof("开始预加载 %d 个店铺配置...", len(s.ownedStores))
		for _, storeID := range s.ownedStores {
			_, err := s.storeAPI.GetStore(storeID)
			if err != nil {
				s.logger.Warnf("预加载店铺 %d 配置失败: %v", storeID, err)
				// 不阻止启动，继续执行
			} else {
				s.logger.Infof("✅ 预加载店铺 %d 配置成功", storeID)
			}
		}
		s.logger.Info("店铺配置预加载完成")
	}

	// 5. 注册消息处理器
	s.registerMessageHandlers()

	// 6. 启动消费者
	if err := s.consumer.Start(s.ctx); err != nil {
		return fmt.Errorf("启动消息消费者失败: %w", err)
	}

	s.started = true
	s.logger.Info("RabbitMQ服务启动完成")
	return nil
}

// registerMessageHandlers 注册消息处理器
func (s *RabbitMQService) registerMessageHandlers() {
	handlers := s.processorRegistry.GetAllHandlers()

	for platform, handler := range handlers {
		isCrawler := platform == "amazon.crawler" || platform == "1688.crawler"

		if isCrawler {
			regions := s.config.Node.Regions
			if len(regions) > 0 {
				for _, region := range regions {
					basePlatform := strings.TrimSuffix(platform, ".crawler")
					queueName := fmt.Sprintf("%s.crawler.%s", basePlatform, strings.ToLower(region))
					s.consumer.RegisterHandler(queueName, handler)
				}
				s.logger.Infof("注册爬虫处理器（按 region）: 平台=%s, regions=%v", platform, regions)
			} else {
				queueName := platform // e.g. "amazon.crawler"
				s.consumer.RegisterHandler(queueName, handler)
				s.logger.Infof("注册爬虫处理器（全局队列）: 平台=%s", platform)
			}
			continue
		}

		if len(s.ownedStores) > 0 {
			for _, storeID := range s.ownedStores {
				queueName := fmt.Sprintf("%s.tasks.store.%d", platform, storeID)
				s.consumer.RegisterHandler(queueName, handler)
			}
			s.logger.Infof("注册处理器: 平台=%s, 店铺=%v", platform, s.ownedStores)
		} else {
			queueName := fmt.Sprintf("%s.tasks", platform)
			s.consumer.RegisterHandler(queueName, handler)
			s.logger.Warnf("注册处理器（平台级队列，未配置店铺隔离）: 平台=%s", platform)
		}
	}

	if len(handlers) == 0 {
		s.logger.Warn("没有注册任何消息处理器")
	}
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

// Stop 停止RabbitMQ服务
func (s *RabbitMQService) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.started {
		s.logger.Info("RabbitMQ服务未启动，无需停止")
		return nil
	}

	s.logger.Info("停止RabbitMQ服务...")

	// 1. 停止消费者
	if err := s.consumer.Stop(ctx); err != nil {
		s.logger.Errorf("停止消费者失败: %v", err)
	}

	// 2. 停止去重器
	if s.deduplicator != nil {
		s.deduplicator.Stop()
	}

	// 3. 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 4. 关闭连接
	if err := s.connManager.Close(); err != nil {
		s.logger.Errorf("关闭RabbitMQ连接失败: %v", err)
	}

	s.started = false
	s.logger.Info("RabbitMQ服务停止完成")
	return nil
}

// IsConnected 检查连接状态
func (s *RabbitMQService) IsConnected() bool {
	return s.connManager.IsConnected()
}

// GetStats 获取服务统计信息
func (s *RabbitMQService) GetStats() map[string]any {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := make(map[string]any)
	stats["started"] = s.started
	stats["connected"] = s.IsConnected()

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
