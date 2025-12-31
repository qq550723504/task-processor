// Package container 提供依赖注入容器
package container

import (
	"context"
	"sync"

	"task-processor/internal/app/service"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/management"
	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/auth"

	"github.com/sirupsen/logrus"
)

// Container 依赖注入容器
type Container struct {
	mu               sync.RWMutex
	logger           *logrus.Logger
	config           *config.Config
	lifecycleManager *lifecycle.Manager

	// 服务实例
	configService    *service.ConfigService
	authService      *service.AuthService
	updaterService   *service.UpdaterService
	processorService service.ProcessorService

	// 共享资源
	amazonProcessor  *amazon.AmazonProcessor
	managementClient *management.ClientManager
	authClient       *auth.ClientCredentialsAuthClient

	// 初始化状态
	initialized bool
}

// NewContainer 创建新的依赖注入容器
func NewContainer(logger *logrus.Logger) *Container {
	return &Container{
		logger:           logger,
		lifecycleManager: lifecycle.NewManager(logger),
	}
}

// Initialize 初始化容器
func (c *Container) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return errors.New(errors.ErrCodeSystem, "容器已经初始化")
	}

	c.logger.Info("开始初始化依赖注入容器...")

	// 初始化服务
	if err := c.initializeServices(); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "初始化服务失败")
	}

	c.initialized = true
	c.logger.Info("依赖注入容器初始化完成")
	return nil
}

// initializeServices 初始化服务
func (c *Container) initializeServices() error {
	// 创建基础服务
	c.configService = service.NewConfigService()
	c.authService = service.NewAuthService(c.logger)
	c.updaterService = service.NewUpdaterService(c.logger)
	c.processorService = service.NewProcessorService(c.logger)

	return nil
}

// LoadConfig 加载配置
func (c *Container) LoadConfig(configPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cfg := c.configService.LoadConfig(configPath)
	if cfg == nil {
		return errors.New(errors.ErrCodeConfig, "配置加载失败")
	}

	if !cfg.ValidateAndLog(c.logger) {
		return errors.New(errors.ErrCodeConfig, "配置验证失败")
	}

	c.config = cfg
	c.logger.Info("配置加载完成")
	return nil
}

// InitializeAuth 初始化认证
func (c *Container) InitializeAuth() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config == nil {
		return errors.New(errors.ErrCodeConfig, "配置未加载")
	}

	authClient, err := c.authService.InitializeClientCredentials(c.config)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeAuth, "初始化认证失败")
	}

	c.authClient = authClient
	c.logger.Info("认证初始化完成")
	return nil
}

// GetSharedAmazonProcessor 获取共享的Amazon处理器
func (c *Container) GetSharedAmazonProcessor() (*amazon.AmazonProcessor, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config == nil {
		return nil, errors.New(errors.ErrCodeConfig, "配置未加载")
	}

	if c.amazonProcessor == nil {
		c.logger.Info("创建共享Amazon处理器...")
		c.amazonProcessor = amazon.NewAmazonProcessor(&c.config.Amazon)
		c.logger.Info("共享Amazon处理器创建完成")
	} else {
		c.logger.Debug("复用现有Amazon处理器")
	}

	return c.amazonProcessor, nil
}

// GetSharedManagementClient 获取共享的管理客户端
func (c *Container) GetSharedManagementClient() (*management.ClientManager, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config == nil {
		return nil, errors.New(errors.ErrCodeConfig, "配置未加载")
	}

	if c.authClient == nil {
		return nil, errors.New(errors.ErrCodeAuth, "认证客户端未初始化")
	}

	if c.managementClient == nil {
		c.logger.Info("创建共享管理客户端...")

		// 获取访问令牌
		accessToken, err := c.authClient.GetAccessToken()
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeAuth, "获取访问令牌失败")
		}

		// 创建管理客户端
		c.managementClient = management.NewClientManager(&c.config.Management)

		// 设置访问令牌
		client := c.managementClient.GetClient()
		client.SetUserToken(accessToken, c.config.Management.TenantID)

		c.logger.Info("共享管理客户端创建完成")
	} else {
		c.logger.Debug("复用现有管理客户端")
	}

	return c.managementClient, nil
}

// StartAll 启动所有组件
func (c *Container) StartAll(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized {
		return errors.New(errors.ErrCodeSystem, "容器未初始化")
	}

	if c.config == nil {
		return errors.New(errors.ErrCodeConfig, "配置未加载")
	}

	if c.authClient == nil {
		return errors.New(errors.ErrCodeAuth, "认证客户端未初始化")
	}

	// 启动自动更新器
	c.updaterService.StartAutoUpdater(c.config, "1.0.0") // TODO: 从外部传入版本号

	// 启动处理器服务
	if err := c.processorService.StartProcessors(ctx, c.config, c.authClient); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动处理器服务失败")
	}

	return nil
}

// StopAll 停止所有组件
func (c *Container) StopAll(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("开始停止所有组件...")

	var lastError error

	// 停止处理器服务
	if c.processorService != nil {
		if err := c.processorService.StopProcessors(); err != nil {
			c.logger.Errorf("停止处理器服务失败: %v", err)
			lastError = err
		}
	}

	// 关闭共享资源
	c.closeSharedResources()

	if lastError != nil {
		return errors.Wrap(lastError, errors.ErrCodeSystem, "停止组件时发生错误")
	}

	c.logger.Info("所有组件停止完成")
	return nil
}

// closeSharedResources 关闭共享资源
func (c *Container) closeSharedResources() {
	// 关闭Amazon处理器
	if c.amazonProcessor != nil {
		c.logger.Info("关闭共享Amazon处理器...")
		c.amazonProcessor.Shutdown()
		c.amazonProcessor = nil
		c.logger.Info("共享Amazon处理器已关闭")
	}

	// 关闭管理客户端
	if c.managementClient != nil {
		c.logger.Info("关闭共享管理客户端...")
		// 注意：management.ClientManager 可能没有Close方法
		// 这里只是清理引用，让GC回收
		c.managementClient = nil
		c.logger.Info("共享管理客户端已关闭")
	}
}

// GetLogger 获取日志器
func (c *Container) GetLogger() *logrus.Logger {
	return c.logger
}

// GetConfig 获取配置
func (c *Container) GetConfig() *config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// GetConfigService 获取配置服务
func (c *Container) GetConfigService() *service.ConfigService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.configService
}

// GetAuthService 获取认证服务
func (c *Container) GetAuthService() *service.AuthService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authService
}

// GetUpdaterService 获取更新服务
func (c *Container) GetUpdaterService() *service.UpdaterService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updaterService
}

// GetProcessorService 获取处理器服务
func (c *Container) GetProcessorService() service.ProcessorService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.processorService
}

// GetAuthClient 获取认证客户端
func (c *Container) GetAuthClient() *auth.ClientCredentialsAuthClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authClient
}
