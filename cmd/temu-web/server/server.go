package server

import (
	"context"
	"time"

	"task-processor/common/auth"
	"task-processor/common/config"
	"task-processor/common/management"
	"task-processor/common/task"
	"task-processor/platforms/shein"
	"task-processor/platforms/temu"

	"github.com/sirupsen/logrus"
)

// Server represents the web server with all its dependencies
type Server struct {
	cfg                     *config.Config
	clientCredentialsClient *auth.ClientCredentialsAuthClient
	sessionManager          *auth.SessionManager
	managementClient        *management.ClientManager
	temuProcessor           *temu.TemuProcessor
	sheinProcessor          *shein.SheinProcessor
	processorCtx            context.Context
	processorCancel         context.CancelFunc
	processorRunning        bool
	logger                  *logrus.Logger
}

// New creates a new server instance
func New(cfg *config.Config, logger *logrus.Logger) *Server {
	return &Server{
		cfg:    cfg,
		logger: logger,
	}
}

// Initialize sets up the server components (deprecated, use InitializeWithClientCredentials)
func (s *Server) Initialize() error {
	s.sessionManager = auth.NewSessionManager()
	return nil
}

// InitializeWithClientCredentials sets up the server with client credentials auth
func (s *Server) InitializeWithClientCredentials(clientCredentialsClient *auth.ClientCredentialsAuthClient) error {
	s.clientCredentialsClient = clientCredentialsClient
	s.sessionManager = auth.NewSessionManager()
	s.autoStartProcessor()
	return nil
}

func (s *Server) autoStartProcessor() {
	s.logger.Info("使用客户端凭证自动启动任务处理器...")

	// 获取访问令牌
	accessToken, err := s.clientCredentialsClient.GetAccessToken()
	if err != nil {
		s.logger.Errorf("获取访问令牌失败: %v", err)
		return
	}

	tenantID := s.clientCredentialsClient.GetTenantID()

	// 初始化任务处理器
	s.initializeTaskProcessor()
	s.setUserTokenToClients(accessToken, tenantID)

	// 启动任务处理器
	go s.startTaskProcessor()

	s.logger.Info("任务处理器自动启动完成")
}

func (s *Server) initializeTaskProcessor() {
	s.logger.Info("创建任务处理器实例...")

	// 创建共享的 managementClient
	s.logger.Info("创建共享的管理客户端...")
	s.managementClient = management.NewClientManager(&s.cfg.Management)

	// 创建 TEMU 处理器（使用共享的 managementClient）
	// TEMU 处理器内部会创建自己的 WorkerPool
	s.logger.Info("创建 TEMU 任务处理器...")
	s.temuProcessor = temu.NewTemuProcessorWithManagementClient(s.cfg, s.logger, s.managementClient)

	// 获取TEMU处理器的共享Amazon处理器
	sharedAmazonProcessor := s.temuProcessor.GetAmazonProcessor()

	// 创建 SHEIN 处理器（使用共享的 managementClient 和 Amazon处理器）
	s.logger.Info("创建 SHEIN 任务处理器...")
	s.sheinProcessor = shein.NewSheinProcessorWithSharedResources(s.cfg, s.managementClient, sharedAmazonProcessor)

	s.processorCtx, s.processorCancel = context.WithCancel(context.Background())
	s.logger.Info("任务处理器实例创建完成（未启动）")
}

func (s *Server) setUserTokenToClients(accessToken, tenantID string) {
	// 使用共享的 managementClient 设置 token
	if s.managementClient != nil {
		client := s.managementClient.GetClient()
		client.SetUserToken(accessToken, tenantID)
		s.logger.Infof("已设置用户令牌到共享管理客户端 (租户: %s)", tenantID)
	}
}

func (s *Server) startTaskProcessor() {
	if s.processorRunning {
		s.logger.Warn("任务处理器已在运行")
		return
	}

	if s.temuProcessor == nil || s.sheinProcessor == nil {
		s.logger.Info("任务处理器未初始化，正在初始化...")
		s.initializeTaskProcessor()
	}

	if s.temuProcessor == nil || s.sheinProcessor == nil || s.processorCtx == nil {
		s.logger.Error("任务处理器组件初始化失败，无法启动")
		return
	}

	s.logger.Info("启动任务处理器...")

	// 启动 TEMU 任务处理器（内部会启动 WorkerPool）
	s.logger.Info("启动 TEMU 任务处理器...")
	if err := s.temuProcessor.Start(s.processorCtx); err != nil {
		s.logger.Errorf("启动 TEMU 任务处理器失败: %v", err)
		s.rollbackStartup()
		return
	}

	// 启动 SHEIN 任务处理器（内部会启动 WorkerPool）
	s.logger.Info("启动 SHEIN 任务处理器...")
	if err := s.sheinProcessor.Start(s.processorCtx); err != nil {
		s.logger.Errorf("启动 SHEIN 任务处理器失败: %v", err)
		// 回滚：关闭已启动的 TEMU 处理器
		s.temuProcessor.Close()
		s.rollbackStartup()
		return
	}

	// 创建任务提交器
	temuSubmitter := temu.NewTemuTaskSubmitter(s.temuProcessor.GetWorkerPool())
	sheinSubmitter := shein.NewSheinTaskSubmitter(s.sheinProcessor.GetWorkerPool())

	// 创建统一任务获取器
	s.logger.Info("启动统一任务获取器...")
	submitters := map[string]task.TaskSubmitter{
		"TEMU":  temuSubmitter,
		"temu":  temuSubmitter,
		"SHEIN": sheinSubmitter,
		"shein": sheinSubmitter,
	}

	// 使用适配器包装管理客户端
	managementProvider := task.WrapManagementClient(s.managementClient)
	unifiedFetcher := task.NewUnifiedTaskFetcher(s.cfg, managementProvider, submitters)

	// 设置任务完成通知器（让WorkerPool在任务完成后通知UnifiedTaskFetcher）
	if temuPool := s.temuProcessor.GetWorkerPool(); temuPool != nil {
		temuPool.SetCompletionNotifier(unifiedFetcher)
		s.logger.Info("已设置TEMU任务完成通知器")
	}
	if sheinPool := s.sheinProcessor.GetWorkerPool(); sheinPool != nil {
		sheinPool.SetCompletionNotifier(unifiedFetcher)
		s.logger.Info("已设置SHEIN任务完成通知器")
	}

	go unifiedFetcher.Start(s.processorCtx)

	s.processorRunning = true
	s.logger.Info("所有任务处理器启动成功 (TEMU + SHEIN + 统一任务获取器)")
}

// rollbackStartup 回滚启动过程
func (s *Server) rollbackStartup() {
	s.logger.Warn("回滚启动过程...")
	if s.processorCancel != nil {
		s.processorCancel()
	}
	s.processorRunning = false
}

func (s *Server) stopTaskProcessor() {
	if !s.processorRunning {
		s.logger.Info("任务处理器未运行，无需停止")
		return
	}

	s.logger.Info("停止任务处理器...")

	// 取消 context，通知所有组件停止
	if s.processorCancel != nil {
		s.processorCancel()
	}

	// 等待一小段时间让组件响应取消信号
	time.Sleep(100 * time.Millisecond)

	// 关闭 TEMU 处理器（内部会关闭 WorkerPool）
	if s.temuProcessor != nil {
		s.logger.Info("停止 TEMU 任务处理器...")
		s.temuProcessor.Close()
	}

	// 关闭 SHEIN 处理器（内部会关闭 WorkerPool）
	if s.sheinProcessor != nil {
		s.logger.Info("停止 SHEIN 任务处理器...")
		s.sheinProcessor.Close()
	}

	s.processorRunning = false
	s.logger.Info("所有任务处理器已停止")
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

func (s *Server) GetClientCredentialsClient() *auth.ClientCredentialsAuthClient {
	return s.clientCredentialsClient
}
