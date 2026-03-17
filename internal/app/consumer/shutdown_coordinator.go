// Package consumer 提供优雅关闭协调器
package consumer

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"task-processor/internal/core/lifecycle"

	"github.com/sirupsen/logrus"
)

// ShutdownCoordinator 监听系统信号并按顺序停止所有已注册组件。
// 组件按注册顺序的逆序停止（后注册的先停止）。
type ShutdownCoordinator struct {
	components      []lifecycle.Component
	shutdownTimeout time.Duration
	logger          *logrus.Logger
}

// NewShutdownCoordinator 创建 ShutdownCoordinator。
// components 按启动顺序传入，关闭时将逆序执行。
func NewShutdownCoordinator(
	components []lifecycle.Component,
	shutdownTimeout time.Duration,
	logger *logrus.Logger,
) *ShutdownCoordinator {
	return &ShutdownCoordinator{
		components:      components,
		shutdownTimeout: shutdownTimeout,
		logger:          logger,
	}
}

// HandleSignals 阻塞等待 SIGINT/SIGTERM，收到信号后执行优雅关闭。
func (s *ShutdownCoordinator) HandleSignals(ctx context.Context, wg *sync.WaitGroup, cancel context.CancelFunc) {
	defer wg.Done()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		s.logger.Infof("收到信号: %v，开始优雅关闭...", sig)
		s.GracefulShutdown(context.Background())
		if cancel != nil {
			cancel()
		}
	case <-ctx.Done():
		s.logger.Info("上下文已取消，停止信号监听")
	}
}

// GracefulShutdown 按逆序停止所有组件。
func (s *ShutdownCoordinator) GracefulShutdown(parentCtx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(parentCtx, s.shutdownTimeout)
	defer cancel()

	s.logger.Info("开始优雅关闭所有服务...")

	for i := len(s.components) - 1; i >= 0; i-- {
		c := s.components[i]
		if !c.IsRunning() {
			continue
		}
		s.logger.Infof("停止组件: %s", c.Name())
		if err := c.Stop(shutdownCtx); err != nil {
			s.logger.Errorf("停止组件 %s 失败: %v", c.Name(), err)
		}
	}

	s.logger.Info("优雅关闭完成")
}
