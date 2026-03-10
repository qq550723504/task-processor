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
	retryStrategy     RetryStrategy

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	closed bool

	// 重连回调
	reconnectCallbacks []func() error
	callbackMutex      sync.RWMutex

	// 状态通知
	stateListeners []ConnectionStateListener
	listenerMutex  sync.RWMutex
	errorCollector *ErrorCollector
}

// ConnectionState 连接状态
type ConnectionState int

const (
	ConnectionStateDisconnected ConnectionState = iota
	ConnectionStateConnecting
	ConnectionStateConnected
	ConnectionStateReconnecting
	ConnectionStateClosed
)

func (cs ConnectionState) String() string {
	switch cs {
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateReconnecting:
		return "reconnecting"
	case ConnectionStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ConnectionStateListener 连接状态监听器
type ConnectionStateListener func(oldState, newState ConnectionState)

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config ConnectionConfig, logger *logrus.Logger) *ConnectionManager {
	// 设置默认值
	config.SetDefaults()

	return &ConnectionManager{
		url:               config.URL,
		logger:            logger,
		reconnectInterval: config.ReconnectInterval,
		maxReconnectTries: config.MaxReconnectTries,
		retryStrategy:     NewDefaultRetryStrategy(config.MaxReconnectTries),
		stateListeners:    make([]ConnectionStateListener, 0),
		errorCollector:    NewErrorCollector(500),
	}
}

// AddStateListener 添加状态监听器
func (cm *ConnectionManager) AddStateListener(listener ConnectionStateListener) {
	cm.listenerMutex.Lock()
	defer cm.listenerMutex.Unlock()
	cm.stateListeners = append(cm.stateListeners, listener)
}

// notifyStateChange 通知状态变更
func (cm *ConnectionManager) notifyStateChange(oldState, newState ConnectionState) {
	cm.listenerMutex.RLock()
	listeners := make([]ConnectionStateListener, len(cm.stateListeners))
	copy(listeners, cm.stateListeners)
	cm.listenerMutex.RUnlock()

	for _, listener := range listeners {
		go listener(oldState, newState)
	}
}

// GetErrorCollector 获取错误收集器
func (cm *ConnectionManager) GetErrorCollector() *ErrorCollector {
	return cm.errorCollector
}

// Connect 建立连接
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.closed {
		return fmt.Errorf("连接管理器已关闭")
	}

	cm.ctx, cm.cancel = context.WithCancel(ctx)

	// 通知状态变更：连接中
	cm.notifyStateChange(ConnectionStateDisconnected, ConnectionStateConnecting)

	// 建立连接
	if err := cm.connect(); err != nil {
		cm.errorCollector.Collect(ErrorTypeConnection, "", "", err, "建立连接失败")
		cm.notifyStateChange(ConnectionStateConnecting, ConnectionStateDisconnected)
		return fmt.Errorf("建立RabbitMQ连接失败: %w", err)
	}

	// 通知状态变更：已连接
	cm.notifyStateChange(ConnectionStateConnecting, ConnectionStateConnected)

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
				cm.errorCollector.Collect(ErrorTypeConnection, "", "", err, "连接意外关闭")
				cm.notifyStateChange(ConnectionStateConnected, ConnectionStateDisconnected)
				cm.reconnect()
			}
		case err := <-chanCloseChan:
			if err != nil {
				cm.logger.Errorf("RabbitMQ通道意外关闭: %v", err)
				cm.errorCollector.Collect(ErrorTypeConnection, "", "", err, "通道意外关闭")
				cm.notifyStateChange(ConnectionStateConnected, ConnectionStateDisconnected)
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

	// 通知状态变更：重连中
	cm.notifyStateChange(ConnectionStateDisconnected, ConnectionStateReconnecting)

	cm.logger.Info("开始重连RabbitMQ...")

	for attempt := 0; attempt < cm.maxReconnectTries; attempt++ {
		select {
		case <-cm.ctx.Done():
			cm.logger.Info("重连被取消")
			return
		default:
		}

		if err := cm.connect(); err != nil {
			// 收集错误
			cm.errorCollector.Collect(ErrorTypeConnection, "", "", err, fmt.Sprintf("重连尝试%d失败", attempt+1))

			// 计算下一次重试的延迟时间
			delay := cm.retryStrategy.NextDelay(attempt)
			cm.logger.Errorf("重连尝试 %d/%d 失败: %v, 等待 %v 后重试",
				attempt+1, cm.maxReconnectTries, err, delay)

			// 释放锁后等待，避免长时间持锁
			cm.mutex.Unlock()
			time.Sleep(delay)
			cm.mutex.Lock()
			continue
		}

		cm.logger.Infof("重连成功，尝试次数: %d", attempt+1)

		// 通知状态变更：已连接
		cm.notifyStateChange(ConnectionStateReconnecting, ConnectionStateConnected)

		// 执行重连回调（重新启动消费者等）
		cm.executeReconnectCallbacks()

		// 重新启动监控
		go cm.monitorConnection()
		return
	}

	cm.logger.Errorf("重连失败，已达到最大尝试次数: %d", cm.maxReconnectTries)
	cm.notifyStateChange(ConnectionStateReconnecting, ConnectionStateDisconnected)
}

// RegisterReconnectCallback 注册重连回调
func (cm *ConnectionManager) RegisterReconnectCallback(callback func() error) {
	cm.callbackMutex.Lock()
	defer cm.callbackMutex.Unlock()
	cm.reconnectCallbacks = append(cm.reconnectCallbacks, callback)
}

// executeReconnectCallbacks 执行重连回调
func (cm *ConnectionManager) executeReconnectCallbacks() {
	cm.callbackMutex.RLock()
	callbacks := make([]func() error, len(cm.reconnectCallbacks))
	copy(callbacks, cm.reconnectCallbacks)
	cm.callbackMutex.RUnlock()

	for i, callback := range callbacks {
		if err := callback(); err != nil {
			cm.logger.Errorf("执行重连回调 %d 失败: %v", i, err)
		}
	}
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

// CreateChannel 创建新的通道（用于消费者独立通道）
func (cm *ConnectionManager) CreateChannel() (*amqp.Channel, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cm.closed {
		return nil, fmt.Errorf("连接管理器已关闭")
	}

	if cm.connection == nil || cm.connection.IsClosed() {
		return nil, fmt.Errorf("RabbitMQ连接不可用")
	}

	// 创建新通道
	channel, err := cm.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("创建新通道失败: %w", err)
	}

	return channel, nil
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

	// 通知状态变更：关闭中
	cm.notifyStateChange(ConnectionStateConnected, ConnectionStateClosed)

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
