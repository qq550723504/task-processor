package server

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/common/auth"
	"task-processor/common/config"
	"task-processor/common/management"
	"task-processor/common/worker"
	"task-processor/platforms/temu"

	"github.com/sirupsen/logrus"
)

// Server represents the web server with all its dependencies
type Server struct {
	cfg              *config.Config
	passwordClient   *auth.PasswordAuthClient
	sessionManager   *auth.SessionManager
	temuProcessor    *temu.TemuProcessor
	workerPool       *worker.Pool
	processorCtx     context.Context
	processorCancel  context.CancelFunc
	processorRunning bool
	templates        embed.FS
	logger           *logrus.Logger
}

// New creates a new server instance
func New(cfg *config.Config, templates embed.FS, logger *logrus.Logger) *Server {
	return &Server{
		cfg:       cfg,
		templates: templates,
		logger:    logger,
	}
}

// Initialize sets up the server components
func (s *Server) Initialize() error {
	s.initializeAuth()
	s.checkAndRestoreSession()
	return nil
}

func (s *Server) setupRoutes() {
	// This will be implemented to set up routes with handlers
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.setupRoutes()
	return s.startServer()
}

func (s *Server) initializeAuth() {
	s.passwordClient = auth.NewPasswordAuthClient(
		s.cfg.Management.BaseURL,
		s.cfg.Management.ClientID,
		s.cfg.Management.ClientSecret,
	)
	s.sessionManager = auth.NewSessionManager()
}

func (s *Server) checkAndRestoreSession() {
	sessions := s.sessionManager.GetAllSessions()
	s.logger.Infof("检查会话恢复: 找到 %d 个会话", len(sessions))

	if len(sessions) > 0 {
		for username, session := range sessions {
			s.logger.Infof("检查会话: 用户=%s, AccessToken=%s..., TenantID=%s",
				username, session.AccessToken[:min(len(session.AccessToken), 10)], session.TenantID)

			if session.AccessToken != "" && session.TenantID != "" {
				if session.Username == "" {
					s.logger.Warn("发现旧的会话数据（用户名为空），跳过自动恢复，请重新登录")
					continue
				}

				s.logger.Infof("发现已保存的会话，自动恢复: 用户=%s, 租户=%s", session.Username, session.TenantID)

				s.initializeTaskProcessor()
				s.setUserTokenToClients(session.AccessToken, session.TenantID)
				go s.startTaskProcessor()
				break
			} else {
				s.logger.Warnf("会话令牌不完整: 用户=%s, AccessToken为空=%t, TenantID为空=%t",
					username, session.AccessToken == "", session.TenantID == "")
			}
		}
	} else {
		s.logger.Info("未找到已保存的会话，需要重新登录")
	}
}

func (s *Server) initializeTaskProcessor() {
	s.logger.Info("创建TEMU任务处理器实例...")
	s.temuProcessor = temu.NewTemuProcessor(s.cfg)
	s.workerPool = worker.NewPool(s.temuProcessor, s.cfg.Worker)
	s.processorCtx, s.processorCancel = context.WithCancel(context.Background())
	s.logger.Info("TEMU任务处理器实例创建完成（未启动）")
}

func (s *Server) setUserTokenToClients(accessToken, tenantID string) {
	if s.temuProcessor != nil {
		s.temuProcessor.SetUserToken(accessToken, tenantID)
	}
}

func (s *Server) startTaskProcessor() {
	if s.processorRunning {
		s.logger.Warn("任务处理器已在运行")
		return
	}

	if s.temuProcessor == nil {
		s.logger.Info("任务处理器未初始化，正在初始化...")
		s.initializeTaskProcessor()
	}

	if s.temuProcessor == nil || s.processorCtx == nil || s.workerPool == nil {
		s.logger.Error("任务处理器组件初始化失败，无法启动")
		return
	}

	s.logger.Info("启动TEMU任务处理器...")

	if err := s.temuProcessor.Start(s.processorCtx); err != nil {
		s.logger.Errorf("启动任务处理器失败: %v", err)
		return
	}

	s.workerPool.Start(s.processorCtx)

	var managementClient *management.Client
	if s.temuProcessor != nil {
		managementClient = s.temuProcessor.GetManagementClient()
	}
	taskFetcher := temu.NewTemuTaskFetcher(s.cfg, s.workerPool, managementClient)
	go taskFetcher.Start(s.processorCtx)

	s.processorRunning = true
	s.logger.Info("TEMU任务处理器启动成功")
}

func (s *Server) stopTaskProcessor() {
	if !s.processorRunning {
		return
	}

	s.logger.Info("停止TEMU任务处理器...")

	if s.processorCancel != nil {
		s.processorCancel()
	}

	if s.workerPool != nil {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.workerPool.Stop(timeoutCtx)
	}

	if s.temuProcessor != nil {
		s.temuProcessor.Close()
	}

	s.processorRunning = false
	s.logger.Info("TEMU任务处理器已停止")
}

// ProcessorManager interface implementation
func (s *Server) IsRunning() bool {
	return s.processorRunning
}

func (s *Server) StartProcessor() error {
	s.startTaskProcessor()
	return nil
}

func (s *Server) StopProcessor() error {
	s.stopTaskProcessor()
	return nil
}

func (s *Server) InitializeProcessor() {
	s.initializeTaskProcessor()
}

func (s *Server) SetUserToken(accessToken, tenantID string) {
	s.setUserTokenToClients(accessToken, tenantID)
}

// Getter methods for exposing components to handlers
func (s *Server) GetSessionManager() *auth.SessionManager {
	return s.sessionManager
}

func (s *Server) GetPasswordClient() *auth.PasswordAuthClient {
	return s.passwordClient
}

func (s *Server) startServer() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)

	s.logger.Infof("配置的服务器端口: %d", s.cfg.Server.Port)
	s.logger.Infof("监听地址: %s", addr)

	webURL := fmt.Sprintf("http://localhost:%d", s.cfg.Server.Port)
	s.logger.Infof("TEMU Web登录界面启动在: %s", webURL)
	s.logger.Info("请在浏览器中打开上述地址进行登录")

	server := &http.Server{Addr: addr}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatalf("启动服务器失败: %v", err)
		}
	}()

	s.logger.Info("等待服务器启动...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	s.logger.Println("收到终止信号，正在关闭...")

	s.stopTaskProcessor()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		s.logger.Errorf("服务器关闭失败: %v", err)
	}

	s.logger.Println("关闭完成")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
