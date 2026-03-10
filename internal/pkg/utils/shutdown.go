// Package utils 提供工具方法
package utils

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"task-processor/internal/pkg/recovery"
	"time"

	"github.com/sirupsen/logrus"
)

// ShutdownManager 优雅关闭管理器
type ShutdownManager struct {
	logger  *logrus.Logger
	hooks   []ShutdownHook
	mutex   sync.Mutex
	timeout time.Duration
	sigChan chan os.Signal
}

// ShutdownHook 关闭钩子函数
type ShutdownHook func(ctx context.Context) error

// NewShutdownManager 创建优雅关闭管理器
func NewShutdownManager(logger *logrus.Logger, timeout time.Duration) *ShutdownManager {
	return &ShutdownManager{
		logger:  logger,
		timeout: timeout,
		sigChan: make(chan os.Signal, 1),
	}
}

// AddHook 添加关闭钩子
func (sm *ShutdownManager) AddHook(hook ShutdownHook) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.hooks = append(sm.hooks, hook)
}

// Wait 等待关闭信号
func (sm *ShutdownManager) Wait() {
	// 监听关闭信号
	signal.Notify(sm.sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// 等待信号
	sig := <-sm.sigChan
	sm.logger.Infof("收到关闭信号: %v，开始优雅关闭...", sig)

	// 执行关闭流程
	sm.shutdown()
}

// shutdown 执行关闭流程
func (sm *ShutdownManager) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), sm.timeout)
	defer cancel()

	sm.mutex.Lock()
	hooks := make([]ShutdownHook, len(sm.hooks))
	copy(hooks, sm.hooks)
	sm.mutex.Unlock()

	// 并发执行所有关闭钩子
	var wg sync.WaitGroup
	errChan := make(chan error, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(index int, h ShutdownHook) {
			defer wg.Done()
			defer recovery.Recover("关闭钩子", sm.logger.WithField("hook_index", index))

			if err := h(ctx); err != nil {
				sm.logger.Errorf("关闭钩子 %d 执行失败: %v", index, err)
				errChan <- err
			} else {
				sm.logger.Infof("关闭钩子 %d 执行成功", index)
			}
		}(i, hook)
	}

	// 等待所有钩子完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		sm.logger.Info("✅ 所有关闭钩子执行完成")
	case <-ctx.Done():
		sm.logger.Warn("⚠️ 关闭超时，强制退出")
	}

	close(errChan)
	errorCount := len(errChan)
	if errorCount > 0 {
		sm.logger.Warnf("⚠️ %d 个关闭钩子执行失败", errorCount)
	}

	sm.logger.Info("✅ 程序已优雅关闭")
	os.Exit(0)
}

// Shutdown 手动触发关闭
func (sm *ShutdownManager) Shutdown() {
	sm.sigChan <- syscall.SIGTERM
}
