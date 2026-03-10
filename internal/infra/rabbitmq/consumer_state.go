// Package rabbitmq 提供RabbitMQ消费者状态管理
package rabbitmq

import (
	"sync"
	"time"
)

// ConsumerState 消费者状态
type ConsumerState int

const (
	// ConsumerStateIdle 空闲状态（未启动）
	ConsumerStateIdle ConsumerState = iota
	// ConsumerStateStarting 启动中
	ConsumerStateStarting
	// ConsumerStateRunning 运行中
	ConsumerStateRunning
	// ConsumerStateStopping 停止中
	ConsumerStateStopping
	// ConsumerStateStopped 已停止
	ConsumerStateStopped
	// ConsumerStateError 错误状态
	ConsumerStateError
)

// String 返回状态的字符串表示
func (s ConsumerState) String() string {
	switch s {
	case ConsumerStateIdle:
		return "idle"
	case ConsumerStateStarting:
		return "starting"
	case ConsumerStateRunning:
		return "running"
	case ConsumerStateStopping:
		return "stopping"
	case ConsumerStateStopped:
		return "stopped"
	case ConsumerStateError:
		return "error"
	default:
		return "unknown"
	}
}

// ConsumerStateInfo 消费者状态信息
type ConsumerStateInfo struct {
	State        ConsumerState // 当前状态
	LastError    error         // 最后一次错误
	StartTime    time.Time     // 启动时间
	StopTime     time.Time     // 停止时间
	ErrorCount   int           // 错误计数
	MessageCount int64         // 处理的消息总数
	SuccessCount int64         // 成功处理的消息数
	FailureCount int64         // 失败处理的消息数
}

// ConsumerStateManager 消费者状态管理器
type ConsumerStateManager struct {
	state      ConsumerState
	stateInfo  ConsumerStateInfo
	mutex      sync.RWMutex
	listeners  []StateChangeListener
	listenerMu sync.RWMutex
}

// StateChangeListener 状态变更监听器
type StateChangeListener func(oldState, newState ConsumerState, queueName string)

// NewConsumerStateManager 创建状态管理器
func NewConsumerStateManager() *ConsumerStateManager {
	return &ConsumerStateManager{
		state: ConsumerStateIdle,
		stateInfo: ConsumerStateInfo{
			State: ConsumerStateIdle,
		},
		listeners: make([]StateChangeListener, 0),
	}
}

// GetState 获取当前状态
func (csm *ConsumerStateManager) GetState() ConsumerState {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	return csm.state
}

// GetStateInfo 获取状态信息（深拷贝）
func (csm *ConsumerStateManager) GetStateInfo() ConsumerStateInfo {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	return csm.stateInfo
}

// SetState 设置状态
func (csm *ConsumerStateManager) SetState(newState ConsumerState, queueName string) {
	csm.mutex.Lock()
	oldState := csm.state
	csm.state = newState
	csm.stateInfo.State = newState

	// 更新时间戳
	now := time.Now()
	switch newState {
	case ConsumerStateRunning:
		csm.stateInfo.StartTime = now
	case ConsumerStateStopped:
		csm.stateInfo.StopTime = now
	}
	csm.mutex.Unlock()

	// 通知监听器（在锁外执行，避免死锁）
	if oldState != newState {
		csm.notifyListeners(oldState, newState, queueName)
	}
}

// SetError 设置错误状态
func (csm *ConsumerStateManager) SetError(err error, queueName string) {
	csm.mutex.Lock()
	oldState := csm.state
	csm.state = ConsumerStateError
	csm.stateInfo.State = ConsumerStateError
	csm.stateInfo.LastError = err
	csm.stateInfo.ErrorCount++
	csm.mutex.Unlock()

	// 通知监听器
	if oldState != ConsumerStateError {
		csm.notifyListeners(oldState, ConsumerStateError, queueName)
	}
}

// IncrementMessageCount 增加消息计数
func (csm *ConsumerStateManager) IncrementMessageCount(success bool) {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	csm.stateInfo.MessageCount++
	if success {
		csm.stateInfo.SuccessCount++
	} else {
		csm.stateInfo.FailureCount++
	}
}

// Reset 重置状态信息
func (csm *ConsumerStateManager) Reset() {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	csm.state = ConsumerStateIdle
	csm.stateInfo = ConsumerStateInfo{
		State: ConsumerStateIdle,
	}
}

// AddListener 添加状态变更监听器
func (csm *ConsumerStateManager) AddListener(listener StateChangeListener) {
	csm.listenerMu.Lock()
	defer csm.listenerMu.Unlock()
	csm.listeners = append(csm.listeners, listener)
}

// notifyListeners 通知所有监听器
func (csm *ConsumerStateManager) notifyListeners(oldState, newState ConsumerState, queueName string) {
	csm.listenerMu.RLock()
	listeners := make([]StateChangeListener, len(csm.listeners))
	copy(listeners, csm.listeners)
	csm.listenerMu.RUnlock()

	for _, listener := range listeners {
		// 在goroutine中执行，避免阻塞
		go listener(oldState, newState, queueName)
	}
}

// IsRunning 检查是否正在运行
func (csm *ConsumerStateManager) IsRunning() bool {
	return csm.GetState() == ConsumerStateRunning
}

// IsHealthy 检查是否健康（运行中且无错误）
func (csm *ConsumerStateManager) IsHealthy() bool {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()
	return csm.state == ConsumerStateRunning && csm.stateInfo.LastError == nil
}
