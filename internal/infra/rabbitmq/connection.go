// Package rabbitmq 提供RabbitMQ连接管理和基础操作
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// ConnectionManager RabbitMQ连接管理器
type ConnectionManager struct {
	url        string
	connection *amqp.Connection
	channel    *amqp.Channel
	mutex      sync.RWMutex
	logger     *logrus.Logger

	// 重连配置
	reconnectInterval time.Duration
	maxReconnectTries int

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	closed bool
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	URL               string        `yaml:"url"`
	ReconnectInterval time.Duration `yaml:"reconnect_interval"`
	MaxReconnectTries int           `yaml:"max_reconnect_tries"`
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config ConnectionConfig, logger *logrus.Logger) *ConnectionManager {
	if config.ReconnectInterval == 0 {
		config.ReconnectInterval = 5 * time.Second
	}
	if config.MaxReconnectTries == 0 {
		config.MaxReconnectTries = 10
	}

	return &ConnectionManager{
		url:               config.URL,
		logger:            logger,
		reconnectInterval: config.ReconnectInterval,
		maxReconnectTries: config.MaxReconnectTries,
	}
}

// Connect 建立连接
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.closed {
		return fmt.Errorf("连接管理器已关闭")
	}

	cm.ctx, cm.cancel = context.WithCancel(ctx)

	// 建立连接
	if err := cm.connect(); err != nil {
		return fmt.Errorf("建立RabbitMQ连接失败: %w", err)
	}

	// 启动连接监控
	go cm.monitorConnection()

	cm.logger.Info("RabbitMQ连接建立成功")
	return nil
}

// connect 内部连接方法
func (cm *ConnectionManager) connect() error {
	var err error

	// 建立连接
	cm.connection, err = amqp.Dial(cm.url)
	if err != nil {
		return fmt.Errorf("连接RabbitMQ失败: %w", err)
	}

	// 创建通道
	cm.channel, err = cm.connection.Channel()
	if err != nil {
		cm.connection.Close()
		return fmt.Errorf("创建RabbitMQ通道失败: %w", err)
	}

	return nil
}

// monitorConnection 监控连接状态
func (cm *ConnectionManager) monitorConnection() {
	defer func() {
		if r := recover(); r != nil {
			cm.logger.Errorf("连接监控发生panic: %v", r)
		}
	}()

	// 监听连接关闭事件
	connCloseChan := make(chan *amqp.Error)
	cm.connection.NotifyClose(connCloseChan)

	// 监听通道关闭事件
	chanCloseChan := make(chan *amqp.Error)
	cm.channel.NotifyClose(chanCloseChan)

	for {
		select {
		case <-cm.ctx.Done():
			cm.logger.Info("连接监控停止")
			return
		case err := <-connCloseChan:
			if err != nil {
				cm.logger.Errorf("RabbitMQ连接意外关闭: %v", err)
				cm.reconnect()
			}
		case err := <-chanCloseChan:
			if err != nil {
				cm.logger.Errorf("RabbitMQ通道意外关闭: %v", err)
				cm.reconnect()
			}
		}
	}
}

// reconnect 重连逻辑
func (cm *ConnectionManager) reconnect() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.closed {
		return
	}

	cm.logger.Info("开始重连RabbitMQ...")

	for i := 0; i < cm.maxReconnectTries; i++ {
		select {
		case <-cm.ctx.Done():
			cm.logger.Info("重连被取消")
			return
		default:
		}

		if err := cm.connect(); err != nil {
			cm.logger.Errorf("重连尝试 %d/%d 失败: %v", i+1, cm.maxReconnectTries, err)
			time.Sleep(cm.reconnectInterval)
			continue
		}

		cm.logger.Infof("重连成功，尝试次数: %d", i+1)

		// 重新启动监控
		go cm.monitorConnection()
		return
	}

	cm.logger.Errorf("重连失败，已达到最大尝试次数: %d", cm.maxReconnectTries)
}

// GetChannel 获取通道
func (cm *ConnectionManager) GetChannel() (*amqp.Channel, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cm.closed {
		return nil, fmt.Errorf("连接管理器已关闭")
	}

	if cm.channel == nil || cm.channel.IsClosed() {
		return nil, fmt.Errorf("RabbitMQ通道不可用")
	}

	return cm.channel, nil
}

// IsConnected 检查连接状态
func (cm *ConnectionManager) IsConnected() bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return cm.connection != nil && !cm.connection.IsClosed() &&
		cm.channel != nil && !cm.channel.IsClosed()
}

// Close 关闭连接
func (cm *ConnectionManager) Close() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.closed {
		return nil
	}

	cm.closed = true

	if cm.cancel != nil {
		cm.cancel()
	}

	var err error
	if cm.channel != nil {
		if channelErr := cm.channel.Close(); channelErr != nil {
			err = fmt.Errorf("关闭通道失败: %w", channelErr)
		}
	}

	if cm.connection != nil {
		if connErr := cm.connection.Close(); connErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; 关闭连接失败: %w", err, connErr)
			} else {
				err = fmt.Errorf("关闭连接失败: %w", connErr)
			}
		}
	}

	cm.logger.Info("RabbitMQ连接已关闭")
	return err
}
