// Package goroutine 提供统一的goroutine管理功能
package goroutine

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// GoroutineManager 统一goroutine管理器
type GoroutineManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     *logrus.Entry
	goroutines map[string]*GoroutineInfo
	mutex      sync.RWMutex
	maxRetries int
}

// GoroutineInfo goroutine信息
type GoroutineInfo struct {
	ID         string
	Name       string
	StartTime  time.Time
	Status     GoroutineStatus
	RetryCount int
	LastError  error
	PanicCount int
}

// GoroutineStatus goroutine状态
type GoroutineStatus int

const (
	StatusRunning GoroutineStatus = iota
	StatusStopped
	StatusError
	StatusPanic
)

func (s GoroutineStatus) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	case StatusPanic:
		return "panic"
	default:
		return "unknown"
	}
}

// GoroutineFunc goroutine执行函数类型
type GoroutineFunc func(ctx context.Context) error

// RetryableGoroutineFunc 可重试的goroutine函数类型
type RetryableGoroutineFunc func(ctx context.Context, retryCount int) error

// NewGoroutineManager 创建goroutine管理器
// 参数:
//   - ctx: 上下文，用于控制管理器的生命周期
//   - logger: 日志记录器，如果为nil则使用默认logger
//
// 返回值:
//   - *GoroutineManager: goroutine管理器实例
func NewGoroutineManager(ctx context.Context, logger *logrus.Entry) *GoroutineManager {
	if logger == nil {
		logger = logrus.WithField("component", "goroutine_manager")
	}

	managerCtx, cancel := context.WithCancel(ctx)

	return &GoroutineManager{
		ctx:        managerCtx,
		cancel:     cancel,
		logger:     logger,
		goroutines: make(map[string]*GoroutineInfo),
		maxRetries: 3,
	}
}

// Start 启动一个goroutine
// 启动一个由管理器管理的goroutine，自动处理panic recovery
// 参数:
//   - name: goroutine的名称，用于标识和日志记录
//   - fn: 要执行的函数
//
// 返回值:
//   - string: goroutine的唯一ID
func (gm *GoroutineManager) Start(name string, fn GoroutineFunc) string {
	return gm.StartWithRetry(name, func(ctx context.Context, retryCount int) error {
		return fn(ctx)
	})
}

// StartWithRetry 启动一个可重试的goroutine
// 启动一个支持自动重试的goroutine，在执行失败时会自动重试
// 参数:
//   - name: goroutine的名称
//   - fn: 要执行的可重试函数，接收重试次数作为参数
//
// 返回值:
//   - string: goroutine的唯一ID
func (gm *GoroutineManager) StartWithRetry(name string, fn RetryableGoroutineFunc) string {
	id := gm.generateID(name)

	info := &GoroutineInfo{
		ID:        id,
		Name:      name,
		StartTime: time.Now(),
		Status:    StatusRunning,
	}

	gm.mutex.Lock()
	gm.goroutines[id] = info
	gm.mutex.Unlock()

	gm.wg.Add(1)
	go gm.runGoroutine(id, fn)

	gm.logger.WithFields(logrus.Fields{
		"goroutine_id":   id,
		"goroutine_name": name,
	}).Info("Goroutine started")

	return id
}

// StartPeriodic 启动周期性执行的goroutine
// 启动一个按指定间隔周期性执行的goroutine
// 参数:
//   - name: goroutine的名称
//   - interval: 执行间隔时间
//   - fn: 要周期性执行的函数
//
// 返回值:
//   - string: goroutine的唯一ID
func (gm *GoroutineManager) StartPeriodic(name string, interval time.Duration, fn GoroutineFunc) string {
	return gm.Start(name, func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if err := fn(ctx); err != nil {
					gm.logger.WithFields(logrus.Fields{
						"goroutine_name": name,
						"error":          err,
					}).Warn("Periodic goroutine execution failed")
				}
			}
		}
	})
}

// Stop 停止指定的goroutine
func (gm *GoroutineManager) Stop(id string) {
	gm.mutex.Lock()
	if info, exists := gm.goroutines[id]; exists {
		info.Status = StatusStopped
	}
	gm.mutex.Unlock()

	gm.logger.WithField("goroutine_id", id).Info("Goroutine stop requested")
}

// StopAll 停止所有goroutine
func (gm *GoroutineManager) StopAll() {
	gm.logger.Info("Stopping all goroutines")
	gm.cancel()
}

// Wait 等待所有goroutine完成
func (gm *GoroutineManager) Wait() {
	gm.wg.Wait()
	gm.logger.Info("All goroutines stopped")
}

// WaitWithTimeout 带超时的等待所有goroutine完成
func (gm *GoroutineManager) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("等待goroutine管理器完成goroutine panic recovered: %v", r)
			}
		}()

		gm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		gm.logger.Info("All goroutines stopped")
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for goroutines to stop")
	}
}

// GetStatus 获取所有goroutine状态
func (gm *GoroutineManager) GetStatus() map[string]*GoroutineInfo {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	status := make(map[string]*GoroutineInfo)
	for id, info := range gm.goroutines {
		// 创建副本避免并发访问问题
		status[id] = &GoroutineInfo{
			ID:         info.ID,
			Name:       info.Name,
			StartTime:  info.StartTime,
			Status:     info.Status,
			RetryCount: info.RetryCount,
			LastError:  info.LastError,
			PanicCount: info.PanicCount,
		}
	}
	return status
}

// GetRunningCount 获取运行中的goroutine数量
func (gm *GoroutineManager) GetRunningCount() int {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	count := 0
	for _, info := range gm.goroutines {
		if info.Status == StatusRunning {
			count++
		}
	}
	return count
}

// runGoroutine 运行goroutine的内部方法
func (gm *GoroutineManager) runGoroutine(id string, fn RetryableGoroutineFunc) {
	defer gm.wg.Done()
	defer gm.updateStatus(id, StatusStopped, nil)

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			// 获取堆栈信息
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			stackTrace := string(buf[:n])

			gm.logger.WithFields(logrus.Fields{
				"goroutine_id": id,
				"panic":        r,
				"stack_trace":  stackTrace,
			}).Error("Goroutine panic recovered")

			gm.incrementPanicCount(id)
			gm.updateStatus(id, StatusPanic, fmt.Errorf("panic: %v", r))
		}
	}()

	retryCount := 0
	for {
		select {
		case <-gm.ctx.Done():
			gm.logger.WithField("goroutine_id", id).Debug("Goroutine stopped by context")
			return
		default:
		}

		err := fn(gm.ctx, retryCount)
		if err == nil {
			// 成功执行，重置重试计数
			gm.resetRetryCount(id)
			return
		}

		// 记录错误
		gm.updateStatus(id, StatusError, err)
		gm.logger.WithFields(logrus.Fields{
			"goroutine_id": id,
			"retry_count":  retryCount,
			"error":        err,
		}).Warn("Goroutine execution failed")

		// 检查是否需要重试
		if retryCount >= gm.maxRetries {
			gm.logger.WithFields(logrus.Fields{
				"goroutine_id": id,
				"max_retries":  gm.maxRetries,
			}).Error("Goroutine max retries exceeded")
			return
		}

		// 等待重试间隔
		retryDelay := time.Duration(retryCount+1) * time.Second
		select {
		case <-gm.ctx.Done():
			return
		case <-time.After(retryDelay):
			retryCount++
			gm.incrementRetryCount(id)
		}
	}
}

// generateID 生成goroutine ID
func (gm *GoroutineManager) generateID(name string) string {
	return fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
}

// updateStatus 更新goroutine状态
func (gm *GoroutineManager) updateStatus(id string, status GoroutineStatus, err error) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if info, exists := gm.goroutines[id]; exists {
		info.Status = status
		info.LastError = err
	}
}

// incrementRetryCount 增加重试计数
func (gm *GoroutineManager) incrementRetryCount(id string) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if info, exists := gm.goroutines[id]; exists {
		info.RetryCount++
	}
}

// resetRetryCount 重置重试计数
func (gm *GoroutineManager) resetRetryCount(id string) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if info, exists := gm.goroutines[id]; exists {
		info.RetryCount = 0
	}
}

// incrementPanicCount 增加panic计数
func (gm *GoroutineManager) incrementPanicCount(id string) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if info, exists := gm.goroutines[id]; exists {
		info.PanicCount++
	}
}

// SetMaxRetries 设置最大重试次数
func (gm *GoroutineManager) SetMaxRetries(maxRetries int) {
	gm.maxRetries = maxRetries
}
