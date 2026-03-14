// Package messaging 提供RabbitMQ服务管理器
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/infra/clients/management/api"

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
	initializer       *QueueInitializer
	processorRegistry *TaskProcessorRegistry
	logger            *logrus.Logger

	// 新增组件
	resultReporter *ResultReporter
	storeAPI       api.StoreAPI
	ownedStores    []int64
	deduplicator   *task.Deduplicator

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

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
	initializer := NewQueueInitializer(client, logger)

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
	deduplicator *task.Deduplicator,
) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.resultReporter = resultReporter
	s.storeAPI = storeAPI
	s.ownedStores = ownedStores
	s.deduplicator = deduplicator

	// 重新创建处理器注册表，使用新的组件
	s.processorRegistry = NewTaskProcessorRegistry(
		resultReporter,
		storeAPI,
		ownedStores,
		deduplicator,
		s.logger,
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
	if err := s.registerMessageHandlers(); err != nil {
		return fmt.Errorf("注册消息处理器失败: %w", err)
	}

	// 6. 启动消费者
	if err := s.consumer.Start(s.ctx); err != nil {
		return fmt.Errorf("启动消息消费者失败: %w", err)
	}

	s.started = true
	s.logger.Info("RabbitMQ服务启动完成")
	return nil
}

// registerMessageHandlers 注册消息处理器
// 每个平台的处理器需要注册到该平台的所有优先级队列（high/normal/low）
func (s *RabbitMQService) registerMessageHandlers() error {
	handlers := s.processorRegistry.GetAllHandlers()

	// 优先级列表
	priorities := []string{"high", "normal", "low"}

	for platform, handler := range handlers {
		// 判断是否是爬虫平台
		isCrawler := platform == "amazon.crawler" || platform == "1688.crawler"

		// 为每个优先级队列注册处理器
		for _, priority := range priorities {
			var queueName string
			if isCrawler {
				// 爬虫队列格式: {platform}.crawler.{priority}
				queueName = fmt.Sprintf("%s.%s", platform, priority)
			} else {
				// 上架任务队列格式: {platform}.tasks.{priority}
				queueName = fmt.Sprintf("%s.tasks.%s", platform, priority)
			}

			s.consumer.RegisterHandler(queueName, handler)
		}

		s.logger.Infof("注册消息处理器: 平台=%s, 队列=3个优先级队列", platform)
	}

	if len(handlers) == 0 {
		s.logger.Warn("没有注册任何消息处理器")
	}

	return nil
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
func (s *RabbitMQService) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := make(map[string]interface{})
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
