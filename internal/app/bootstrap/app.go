// Package bootstrap 提供应用启动器实现
package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/app/di"

	"github.com/sirupsen/logrus"
)

// ApplicationBootstrap 应用启动器
type ApplicationBootstrap struct {
	logger           *logrus.Logger
	container        di.Container
	configManager    config.ConfigManager
	lifecycleManager lifecycle.LifecycleManager
	serviceRegistry  *ServiceRegistrySimple
	appVersion       string
}

// NewApplicationBootstrap 创建应用启动器
func NewApplicationBootstrap(logger *logrus.Logger) *ApplicationBootstrap {
	return &ApplicationBootstrap{
		logger:           logger,
		container:        di.NewContainer(),
		configManager:    config.NewConfigManager(logger),
		lifecycleManager: lifecycle.NewLifecycleManager(logger),
		serviceRegistry:  NewServiceRegistrySimple(logger),
	}
}

// Initialize 初始化应用
func (a *ApplicationBootstrap) Initialize(configPath, appVersion string) error {
	a.logger.Info("开始初始化应用...")

	// 保存版本信息
	a.appVersion = appVersion

	// 1. 加载配置
	if err := a.loadConfiguration(configPath); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 2. 注册核心服务到容器
	if err := a.registerCoreServices(); err != nil {
		return fmt.Errorf("注册核心服务失败: %w", err)
	}

	// 3. 注册业务服务到容器
	if err := a.registerBusinessServices(); err != nil {
		return fmt.Errorf("注册业务服务失败: %w", err)
	}

	// 4. 创建组件适配器并注册到生命周期管理器
	if err := a.registerComponents(); err != nil {
		return fmt.Errorf("注册组件失败: %w", err)
	}

	a.logger.Info("✅ 应用初始化完成")
	return nil
}

// Start 启动应用
func (a *ApplicationBootstrap) Start(ctx context.Context, appVersion string) error {
	a.logger.Info("开始启动应用...")

	// 启动所有组件
	if err := a.lifecycleManager.StartAll(ctx); err != nil {
		return fmt.Errorf("启动组件失败: %w", err)
	}

	a.logger.Info("✅ 应用启动完成")
	return nil
}

// Stop 停止应用
func (a *ApplicationBootstrap) Stop(ctx context.Context) error {
	a.logger.Info("开始停止应用...")

	// 停止所有组件
	if err := a.lifecycleManager.StopAll(ctx); err != nil {
		a.logger.Errorf("停止组件失败: %v", err)
	}

	// 关闭容器
	if err := a.container.Close(); err != nil {
		a.logger.Errorf("关闭容器失败: %v", err)
	}

	a.logger.Info("✅ 应用停止完成")
	return nil
}

// GetContainer 获取依赖注入容器
func (a *ApplicationBootstrap) GetContainer() di.Container {
	return a.container
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
	a.logger.Info("加载应用配置...")
	a.logger.Infof("配置文件路径: %s", configPath)

	// 创建配置源
	source := config.NewFileConfigSource(configPath)

	// 加载配置
	cfg, err := a.configManager.Load(source)
	if err != nil {
		return err
	}

	a.logger.Infof("配置加载成功，来源: %s", source.Name())

	// 添加调试信息，验证配置是否正确加载
	a.logger.Infof("浏览器配置 - 启用: %v, 路径: %s, 池大小: %d",
		cfg.Browser.Enabled, cfg.Browser.BrowserPath, cfg.Browser.PoolSize)
	a.logger.Infof("管理系统配置 - URL: %s, 客户端ID: %s",
		cfg.Management.BaseURL, cfg.Management.ClientID)

	_ = cfg // 配置已经存储在configManager中
	return nil
}

// registerCoreServices 注册核心服务
func (a *ApplicationBootstrap) registerCoreServices() error {
	a.logger.Info("注册核心服务...")

	// 注册配置管理器
	if err := a.container.RegisterSingleton("configManager", func(c di.Container) (any, error) {
		return a.configManager, nil
	}); err != nil {
		return err
	}

	// 注册配置对象
	if err := a.container.RegisterSingleton("config", func(c di.Container) (any, error) {
		return a.configManager.GetCurrent(), nil
	}); err != nil {
		return err
	}

	// 注册生命周期管理器
	if err := a.container.RegisterSingleton("lifecycleManager", func(c di.Container) (any, error) {
		return a.lifecycleManager, nil
	}); err != nil {
		return err
	}

	// 注册日志器
	if err := a.container.RegisterSingleton("logger", func(c di.Container) (any, error) {
		return a.logger, nil
	}); err != nil {
		return err
	}

	a.logger.Info("✅ 核心服务注册完成")
	return nil
}

// registerBusinessServices 注册业务服务
func (a *ApplicationBootstrap) registerBusinessServices() error {
	a.logger.Info("注册业务服务...")

	cfg := a.configManager.GetCurrent()
	if cfg == nil {
		return fmt.Errorf("配置未加载")
	}

	// 使用服务注册表注册所有业务服务
	if err := a.serviceRegistry.RegisterAllServices(a.container, cfg); err != nil {
		return err
	}

	a.logger.Info("✅ 业务服务注册完成")
	return nil
}

// registerComponents 注册组件到生命周期管理器
func (a *ApplicationBootstrap) registerComponents() error {
	a.logger.Info("注册组件到生命周期管理器...")

	cfg := a.configManager.GetCurrent()
	if cfg == nil {
		return fmt.Errorf("配置未加载")
	}

	// 创建组件适配器
	adapters := NewComponentAdapters(a.container, a.logger)

	// 注册所有组件适配器，传递版本信息
	if err := adapters.RegisterAllComponents(a.lifecycleManager, cfg, a.appVersion); err != nil {
		return err
	}

	a.logger.Info("✅ 组件注册完成")
	return nil
}
