// Package amazon 提供超时管理功能
package amazon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// TimeoutManager 超时管理器
type TimeoutManager struct {
	defaultTimeout time.Duration
	logger         *logrus.Entry
	mu             sync.RWMutex
	activeContexts map[string]context.CancelFunc
}

// NewTimeoutManager 创建超时管理器
func NewTimeoutManager(defaultTimeout time.Duration) *TimeoutManager {
	return &TimeoutManager{
		defaultTimeout: defaultTimeout,
		logger:         logger.GetGlobalLogger("TimeoutManager"),
		activeContexts: make(map[string]context.CancelFunc),
	}
}

// CreateTimeoutContext 创建带超时的上下文
func (tm *TimeoutManager) CreateTimeoutContext(parent context.Context, taskID string, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = tm.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(parent, timeout)

	// 包装cancel函数以便清理
	wrappedCancel := func() {
		tm.mu.Lock()
		delete(tm.activeContexts, taskID)
		tm.mu.Unlock()
		cancel()
	}

	tm.mu.Lock()
	tm.activeContexts[taskID] = wrappedCancel
	tm.mu.Unlock()

	tm.logger.Infof("创建超时上下文: TaskID=%s, Timeout=%v", taskID, timeout)
	return ctx, wrappedCancel
}

// CancelAll 取消所有活跃的上下文
func (tm *TimeoutManager) CancelAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for taskID, cancel := range tm.activeContexts {
		tm.logger.Warnf("强制取消任务: TaskID=%s", taskID)
		cancel()
	}
	tm.activeContexts = make(map[string]context.CancelFunc)
}

// GetActiveCount 获取活跃上下文数量
func (tm *TimeoutManager) GetActiveCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.activeContexts)
}

// ProcessWithTimeout 带超时执行处理函数
func (tm *TimeoutManager) ProcessWithTimeout(
	ctx context.Context,
	taskID string,
	timeout time.Duration,
	processFunc func(context.Context) error,
) error {
	timeoutCtx, cancel := tm.CreateTimeoutContext(ctx, taskID, timeout)
	defer cancel()

	// 使用channel来处理结果和超时
	resultChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- fmt.Errorf("处理函数发生panic: %v", r)
			}
		}()

		err := processFunc(timeoutCtx)
		resultChan <- err
	}()

	select {
	case err := <-resultChan:
		return err
	case <-timeoutCtx.Done():
		tm.logger.Errorf("任务超时: TaskID=%s, Timeout=%v", taskID, timeout)
		return fmt.Errorf("任务执行超时: %v", timeout)
	}
}
