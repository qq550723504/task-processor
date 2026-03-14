// Package shutdown 提供优雅关闭管理
package shutdown

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

// Hook 关闭钩子函数
type Hook func(ctx context.Context) error

// Manager 优雅关闭管理器
type Manager struct {
	logger  *logrus.Logger
	hooks   []Hook
	mutex   sync.Mutex
	timeout time.Duration
	sigChan chan os.Signal
}

// NewManager 创建优雅关闭管理器
func NewManager(logger *logrus.Logger, timeout time.Duration) *Manager {
	return &Manager{
		logger:  logger,
		timeout: timeout,
		sigChan: make(chan os.Signal, 1),
	}
}

// AddHook 添加关闭钩子
func (m *Manager) AddHook(hook Hook) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.hooks = append(m.hooks, hook)
}

// Wait 等待关闭信号
func (m *Manager) Wait() {
	signal.Notify(m.sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-m.sigChan
	m.logger.Infof("收到关闭信号: %v，开始优雅关闭...", sig)
	m.shutdown()
}

// Shutdown 手动触发关闭
func (m *Manager) Shutdown() {
	m.sigChan <- syscall.SIGTERM
}

func (m *Manager) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.mutex.Lock()
	hooks := make([]Hook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mutex.Unlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(index int, h Hook) {
			defer wg.Done()
			defer recovery.Recover("关闭钩子", m.logger.WithField("hook_index", index))
			if err := h(ctx); err != nil {
				m.logger.Errorf("关闭钩子 %d 执行失败: %v", index, err)
				errChan <- err
			} else {
				m.logger.Infof("关闭钩子 %d 执行成功", index)
			}
		}(i, hook)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("✅ 所有关闭钩子执行完成")
	case <-ctx.Done():
		m.logger.Warn("⚠️ 关闭超时，强制退出")
	}

	close(errChan)
	if n := len(errChan); n > 0 {
		m.logger.Warnf("⚠️ %d 个关闭钩子执行失败", n)
	}

	m.logger.Info("✅ 程序已优雅关闭")
	os.Exit(0)
}
