// Package bootstrap 提供应用启动器实现
package bootstrap

import (
	"context"
	"fmt"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

// appServices 持有所有已构造好的服务实例
type appServices struct {
	cfg              *config.Config
	authClient       *auth.ClientCredentialsAuthClient
	managementClient *management.ClientManager
	amazonCrawler    *amazon.AmazonProcessor
	temuProcessor    *temu.TemuProcessor
	sheinProcessor   *pipeline.SheinProcessor
	processorService runner.ProcessorService
	schedulerService runner.SchedulerService
}

// ApplicationBootstrap 应用启动器
type ApplicationBootstrap struct {
	logger           *logrus.Logger
	configManager    config.ConfigManager
	lifecycleManager lifecycle.LifecycleManager
	services         *appServices
	appVersion       string
}

// NewApplicationBootstrap 创建应用启动器
func NewApplicationBootstrap(logger *logrus.Logger) *ApplicationBootstrap {
	return &ApplicationBootstrap{
		logger:           logger,
		configManager:    config.NewConfigManager(logger),
		lifecycleManager: lifecycle.NewLifecycleManager(logger),
	}
}

// Initialize 初始化应用
func (a *ApplicationBootstrap) Initialize(configPath, appVersion string) error {
	a.logger.Info("开始初始化应用...")
	a.appVersion = appVersion

	if err := a.loadConfiguration(configPath); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	svc, err := buildServices(a.configManager.GetCurrent(), a.logger)
	if err != nil {
		return fmt.Errorf("构建服务失败: %w", err)
	}
	a.services = svc

	if err := registerComponents(a.lifecycleManager, a.services, a.logger, a.appVersion); err != nil {
		return fmt.Errorf("注册组件失败: %w", err)
	}

	a.logger.Info("✅ 应用初始化完成")
	return nil
}

// Start 启动应用
func (a *ApplicationBootstrap) Start(ctx context.Context, appVersion string) error {
	a.logger.Info("开始启动应用...")
	if err := a.lifecycleManager.StartAll(ctx); err != nil {
		return fmt.Errorf("启动组件失败: %w", err)
	}
	a.logger.Info("✅ 应用启动完成")
	return nil
}

// Stop 停止应用
func (a *ApplicationBootstrap) Stop(ctx context.Context) error {
	a.logger.Info("开始停止应用...")
	if err := a.lifecycleManager.StopAll(ctx); err != nil {
		a.logger.Errorf("停止组件失败: %v", err)
	}
	a.logger.Info("✅ 应用停止完成")
	return nil
}

// GetConfigManager 获取配置管理器
func (a *ApplicationBootstrap) GetConfigManager() config.ConfigManager {
	return a.configManager
}

// GetLifecycleManager 获取生命周期管理器
func (a *ApplicationBootstrap) GetLifecycleManager() lifecycle.LifecycleManager {
	return a.lifecycleManager
}

// loadConfiguration 加载配置
func (a *ApplicationBootstrap) loadConfiguration(configPath string) error {
	a.logger.Infof("加载配置文件: %s", configPath)
	source := config.NewFileConfigSource(configPath)
	cfg, err := a.configManager.Load(source)
	if err != nil {
		return err
	}
	a.logger.Infof("浏览器配置 - 启用: %v, 路径: %s, 池大小: %d",
		cfg.Browser.Enabled, cfg.Browser.BrowserPath, cfg.Browser.PoolSize)
	a.logger.Infof("管理系统配置 - URL: %s, 客户端ID: %s",
		cfg.Management.BaseURL, cfg.Management.ClientID)
	return nil
}

// buildServices 构造所有服务实例（直接依赖注入，无容器）
func buildServices(cfg *config.Config, logger *logrus.Logger) (*appServices, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置未加载")
	}

	// 认证客户端
	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		tenantID = "1"
	}
	authClient := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		tenantID,
		logger,
	)
	if _, err := authClient.GetAccessToken(); err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 管理客户端
	managementClient := management.NewClientManager(&cfg.Management)
	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}
	managementClient.GetClient().SetUserToken(accessToken, cfg.Management.TenantID)

	// Amazon 爬虫
	amazonCrawler := amazon.NewAmazonProcessor(cfg)

	return &appServices{
		cfg:              cfg,
		authClient:       authClient,
		managementClient: managementClient,
		amazonCrawler:    amazonCrawler,
		processorService: runner.NewProcessorServiceWithDependencies(logger, managementClient, amazonCrawler),
		schedulerService: runner.NewSchedulerServiceWithAmazon(logger, managementClient, cfg, amazonCrawler, nil),
	}, nil
}
