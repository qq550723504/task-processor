// Package rabbitmq 提供RabbitMQ服务管理器
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/app/worker"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// Service RabbitMQ服务管理器
type Service struct {
	config            *RabbitMQConfig
	connManager       *ConnectionManager
	client            *Client
	consumer          *MessageConsumer
	initializer       *QueueInitializer
	processorRegistry *TaskProcessorRegistry
	logger            *logrus.Logger

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态管理
	started bool
	mutex   sync.RWMutex
}

// RabbitMQConfig RabbitMQ配置
type RabbitMQConfig struct {
	URL               string         `yaml:"url"`
	ReconnectInterval time.Duration  `yaml:"reconnect_interval"`
	MaxReconnectTries int            `yaml:"max_reconnect_tries"`
	Consumer          ConsumerConfig `yaml:"consumer"`
}

// NewService 创建RabbitMQ服务
func NewService(config *RabbitMQConfig, logger *logrus.Logger) *Service {
	// 设置默认值
	if config.ReconnectInterval == 0 {
		config.ReconnectInterval = 5 * time.Second
	}
	if config.MaxReconnectTries == 0 {
		config.MaxReconnectTries = 10
	}
	if config.Consumer.PrefetchCount == 0 {
		config.Consumer.PrefetchCount = 1
	}
	if config.Consumer.RetryDelay == 0 {
		config.Consumer.RetryDelay = 5 * time.Second
	}
	if config.Consumer.MaxRetries == 0 {
		config.Consumer.MaxRetries = 3
	}

	// 创建连接管理器
	connConfig := ConnectionConfig{
		URL:               config.URL,
		ReconnectInterval: config.ReconnectInterval,
		MaxReconnectTries: config.MaxReconnectTries,
	}
	connManager := NewConnectionManager(connConfig, logger)

	// 创建客户端
	client := NewClient(connManager, logger)

	// 创建消费者
	consumer := NewMessageConsumer(client, config.Consumer, logger)

	// 创建初始化器
	initializer := NewQueueInitializer(client, logger)

	// 创建处理器注册表
	processorRegistry := NewTaskProcessorRegistry(logger)

	return &Service{
		config:            config,
		connManager:       connManager,
		client:            client,
		consumer:          consumer,
		initializer:       initializer,
		processorRegistry: processorRegistry,
		logger:            logger,
	}
}

// RegisterProcessor 注册任务处理器
func (s *Service) RegisterProcessor(platform string, processor worker.Processor) error {
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

// Start 启动RabbitMQ服务
func (s *Service) Start(ctx context.Context) error {
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

	// 2. 初始化队列和交换机
	if err := s.initializer.InitializeAll(); err != nil {
		return fmt.Errorf("初始化RabbitMQ队列失败: %w", err)
	}

	// 3. 注册消息处理器
	if err := s.registerMessageHandlers(); err != nil {
		return fmt.Errorf("注册消息处理器失败: %w", err)
	}

	// 4. 启动消费者
	if err := s.consumer.Start(s.ctx); err != nil {
		return fmt.Errorf("启动消息消费者失败: %w", err)
	}

	s.started = true
	s.logger.Info("RabbitMQ服务启动完成")
	return nil
}

// registerMessageHandlers 注册消息处理器
func (s *Service) registerMessageHandlers() error {
	handlers := s.processorRegistry.GetAllHandlers()

	for platform, handler := range handlers {
		queueName := s.processorRegistry.GetQueueName(platform)
		s.consumer.RegisterHandler(queueName, handler)
		s.logger.Infof("注册消息处理器: 平台=%s, 队列=%s", platform, queueName)
	}

	if len(handlers) == 0 {
		s.logger.Warn("没有注册任何消息处理器")
	}

	return nil
}

// Stop 停止RabbitMQ服务
func (s *Service) Stop(ctx context.Context) error {
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

	// 2. 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 3. 关闭连接
	if err := s.connManager.Close(); err != nil {
		s.logger.Errorf("关闭RabbitMQ连接失败: %v", err)
	}

	s.started = false
	s.logger.Info("RabbitMQ服务停止完成")
	return nil
}

// IsConnected 检查连接状态
func (s *Service) IsConnected() bool {
	return s.connManager.IsConnected()
}

// GetStats 获取服务统计信息
func (s *Service) GetStats() map[string]interface{} {
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
func (s *Service) GetClient() *Client {
	return s.client
}

// GetConsumer 获取消息消费者
func (s *Service) GetConsumer() *MessageConsumer {
	return s.consumer
}

// ServiceBuilder RabbitMQ服务构建器
type ServiceBuilder struct {
	config     *RabbitMQConfig
	processors map[string]worker.Processor
	logger     *logrus.Logger
}

// NewServiceBuilder 创建服务构建器
func NewServiceBuilder(logger *logrus.Logger) *ServiceBuilder {
	return &ServiceBuilder{
		processors: make(map[string]worker.Processor),
		logger:     logger,
	}
}

// WithConfig 设置配置
func (sb *ServiceBuilder) WithConfig(config *RabbitMQConfig) *ServiceBuilder {
	sb.config = config
	return sb
}

// WithConfigFromAppConfig 从应用配置创建RabbitMQ配置
func (sb *ServiceBuilder) WithConfigFromAppConfig(appConfig *config.Config) *ServiceBuilder {
	// 从应用配置中提取RabbitMQ配置
	// 这里需要根据实际的配置结构调整
	rabbitmqConfig := &RabbitMQConfig{
		URL:               "amqp://guest:guest@localhost:5672/", // 默认值
		ReconnectInterval: 5 * time.Second,
		MaxReconnectTries: 10,
		Consumer: ConsumerConfig{
			PrefetchCount: 1,
			PrefetchSize:  0,
			RetryDelay:    5 * time.Second,
			MaxRetries:    3,
		},
	}

	// 如果应用配置中有RabbitMQ配置，则使用它
	// TODO: 根据实际配置结构调整

	sb.config = rabbitmqConfig
	return sb
}

// WithProcessor 添加处理器
func (sb *ServiceBuilder) WithProcessor(platform string, processor worker.Processor) *ServiceBuilder {
	sb.processors[platform] = processor
	return sb
}

// Build 构建服务
func (sb *ServiceBuilder) Build() (*Service, error) {
	if sb.config == nil {
		return nil, fmt.Errorf("配置不能为空")
	}

	if sb.logger == nil {
		return nil, fmt.Errorf("日志器不能为空")
	}

	// 创建服务
	service := NewService(sb.config, sb.logger)

	// 注册处理器
	for platform, processor := range sb.processors {
		if err := service.RegisterProcessor(platform, processor); err != nil {
			return nil, fmt.Errorf("注册处理器 %s 失败: %w", platform, err)
		}
	}

	return service, nil
}
