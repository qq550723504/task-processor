package messaging

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// ShutdownCoordinator 负责信号监听和优雅关闭逻辑
type ShutdownCoordinator struct {
	config           *config.RabbitMQConfig
	rabbitmqService  *RabbitMQService
	httpServerManger *HTTPServerManager
	resultReporter   *ResultReporter
	loadMonitor      *rabbitmq.LoadMonitor
	logger           *logrus.Logger
}

// NewShutdownCoordinator 创建 ShutdownCoordinator
func NewShutdownCoordinator(
	cfg *config.RabbitMQConfig,
	rabbitmqService *RabbitMQService,
	httpServerManger *HTTPServerManager,
	resultReporter *ResultReporter,
	loadMonitor *rabbitmq.LoadMonitor,
	logger *logrus.Logger,
) *ShutdownCoordinator {
	return &ShutdownCoordinator{
		config:           cfg,
		rabbitmqService:  rabbitmqService,
		httpServerManger: httpServerManger,
		resultReporter:   resultReporter,
		loadMonitor:      loadMonitor,
		logger:           logger,
	}
}

// HandleSignals 监听系统信号并在收到信号时触发优雅关闭
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

// GracefulShutdown 执行优雅关闭逻辑
func (s *ShutdownCoordinator) GracefulShutdown(parentCtx context.Context) {
	shutdownCtx, shutdownCancel := context.WithTimeout(parentCtx, s.config.Node.ShutdownTimeout)
	defer shutdownCancel()

	s.logger.Info("开始优雅关闭所有服务...")

	// 停止接收新任务
	if s.rabbitmqService != nil {
		if err := s.rabbitmqService.Stop(shutdownCtx); err != nil {
			s.logger.Errorf("停止RabbitMQ服务失败: %v", err)
		}
	}

	// 停止HTTP服务器
	if s.httpServerManger != nil {
		if err := s.httpServerManger.Stop(shutdownCtx); err != nil {
			s.logger.Errorf("停止HTTP服务器失败: %v", err)
		}
	}

	// 停止其他服务
	if s.resultReporter != nil {
		if err := s.resultReporter.Stop(shutdownCtx); err != nil {
			s.logger.Errorf("停止结果上报器失败: %v", err)
		}
	}

	if s.loadMonitor != nil {
		if err := s.loadMonitor.Stop(shutdownCtx); err != nil {
			s.logger.Errorf("停止负载监控失败: %v", err)
		}
	}

	s.logger.Info("优雅关闭完成")
}
