// Package service 提供业务逻辑层
package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"task-processor/cmd/temu-web/server"
	"task-processor/common/auth"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// ServerService 服务器服务
type ServerService struct {
	logger *logrus.Logger
	server *server.Server
}

// NewServerService 创建服务器服务实例
func NewServerService(logger *logrus.Logger) *ServerService {
	return &ServerService{
		logger: logger,
	}
}

// InitializeServer 初始化服务器
func (s *ServerService) InitializeServer(cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	// 创建服务器实例
	s.server = server.New(cfg, s.logger)

	// 使用客户端凭证初始化服务器组件
	if err := s.server.InitializeWithClientCredentials(authClient); err != nil {
		return fmt.Errorf("服务器初始化失败: %w", err)
	}

	return nil
}

// StartServer 启动服务器并等待信号
func (s *ServerService) StartServer() {
	s.logger.Info("✅ 任务处理器正在运行中...")
	s.logger.Info("按 Ctrl+C 停止程序")

	// 设置优雅关闭
	s.setupGracefulShutdown()
}

// setupGracefulShutdown 设置优雅关闭
func (s *ServerService) setupGracefulShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	s.logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 停止任务处理器
	if err := s.server.StopProcessor(); err != nil {
		s.logger.Errorf("停止任务处理器失败: %v", err)
	}

	s.logger.Info("✅ 程序已优雅关闭")
	os.Exit(0)
}
