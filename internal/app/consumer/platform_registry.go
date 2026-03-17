// Package consumer 提供多平台处理器注册功能
package consumer

import (
	"context"
	"fmt"
	"strings"

	platformAmazon "task-processor/internal/amazon"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

// PlatformRegistry 多平台处理器注册器
type PlatformRegistry struct {
	config                *config.Config
	logger                *logrus.Logger
	managementClient      *management.ClientManager
	sharedAmazonProcessor *amazon.AmazonProcessor
	rabbitmqClient        *rabbitmq.Client
	enabledPlatforms      []string
}

// NewPlatformRegistry 创建平台注册器
func NewPlatformRegistry(
	cfg *config.Config,
	logger *logrus.Logger,
	platformsStr string,
) *PlatformRegistry {
	// 如果指定了平台列表，使用指定的；否则从配置文件读取
	var enabledPlatforms []string
	if platformsStr != "" {
		enabledPlatforms = parsePlatformList(platformsStr)
	} else {
		enabledPlatforms = getEnabledPlatformsFromConfig(cfg)
	}

	logger.Infof(" 启用的平台: %v", enabledPlatforms)

	return &PlatformRegistry{
		config:           cfg,
		logger:           logger,
		enabledPlatforms: enabledPlatforms,
	}
}

// RegisterAllProcessors 注册所有启用的平台处理器
func (r *PlatformRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager) error {
	r.logger.Info(" 开始注册平台处理器...")

	// 获取RabbitMQ客户端（用于分布式爬虫）
	r.rabbitmqClient = serviceManager.GetClient()
	if r.rabbitmqClient != nil {
		r.logger.Info(" 获取到RabbitMQ客户端，将启用分布式爬虫")
	} else {
		r.logger.Warn(" 未获取到RabbitMQ客户端，将使用本地爬虫")
	}

	// 初始化共享资源
	if err := r.initializeSharedResources(); err != nil {
		return fmt.Errorf("初始化共享资源失败: %w", err)
	}

	// 注册各个平台
	if err := r.registerAmazonPlatform(ctx, serviceManager); err != nil {
		return err
	}

	if err := r.registerTemuPlatform(ctx, serviceManager); err != nil {
		return err
	}

	if err := r.registerSheinPlatform(ctx, serviceManager); err != nil {
		return err
	}

	r.logger.Info(" 所有平台处理器注册完成")
	return nil
}

// initializeSharedResources 初始化管理客户端和共享的 Amazon 处理器。
func (r *PlatformRegistry) initializeSharedResources() error {
	r.managementClient = management.NewClientManager(&r.config.Management)
	if err := r.applyAccessToken(); err != nil {
		return err
	}
	if r.needsAmazonProcessor() {
		r.sharedAmazonProcessor = amazon.CreateProcessor(r.config, r.logger)
	}
	r.logger.Info("共享资源初始化完成")
	return nil
}

// applyAccessToken 获取访问令牌并注入到管理客户端。
func (r *PlatformRegistry) applyAccessToken() error {
	authClient := auth.NewClientCredentialsAuthClient(
		r.config.Management.BaseURL,
		r.config.Management.ClientID,
		r.config.Management.ClientSecret,
		r.config.Management.TenantID,
		r.logger,
	)
	token, err := authClient.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取访问令牌失败: %w", err)
	}
	r.managementClient.GetClient().SetUserToken(token, r.config.Management.TenantID)
	return nil
}

// needsAmazonProcessor 检查是否需要Amazon处理器
func (r *PlatformRegistry) needsAmazonProcessor() bool {
	return containsPlatform(r.enabledPlatforms, "temu") ||
		containsPlatform(r.enabledPlatforms, "shein")
}

// registerAmazonPlatform 注册Amazon平台处理器
func (r *PlatformRegistry) registerAmazonPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "amazon") {
		r.logger.Debug("跳过Amazon平台注册")
		return nil
	}

	r.logger.Info(" 注册Amazon平台处理器...")

	// 使用完整的Amazon平台处理器（用于上架）
	amazonProcessor := platformAmazon.NewProcessor(ctx, r.config, r.logger)

	if err := serviceManager.RegisterProcessor("amazon", amazonProcessor); err != nil {
		return fmt.Errorf("注册Amazon平台处理器失败: %w", err)
	}

	r.logger.Info(" Amazon平台处理器注册成功")
	return nil
}

// registerTemuPlatform 注册TEMU平台处理器
func (r *PlatformRegistry) registerTemuPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "temu") {
		r.logger.Debug("跳过TEMU平台注册")
		return nil
	}

	r.logger.Info(" 注册TEMU处理器...")

	temuProcessor, err := temu.NewTemuProcessor(
		ctx,
		r.config,
		r.logger,
		r.managementClient,
		r.sharedAmazonProcessor,
		r.rabbitmqClient,
	)
	if err != nil {
		return fmt.Errorf("创建TEMU处理器失败: %w", err)
	}

	if err := serviceManager.RegisterProcessor("temu", temuProcessor); err != nil {
		return fmt.Errorf("注册TEMU处理器失败: %w", err)
	}

	r.logger.Info(" TEMU处理器注册成功")
	return nil
}

// registerSheinPlatform 注册SHEIN平台处理器
func (r *PlatformRegistry) registerSheinPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "shein") {
		r.logger.Debug("跳过SHEIN平台注册")
		return nil
	}

	r.logger.Info(" 注册SHEIN处理器...")

	sheinProcessor, err := pipeline.NewSheinProcessor(
		ctx,
		r.config,
		r.logger,
		r.managementClient,
		r.sharedAmazonProcessor,
		r.rabbitmqClient,
	)
	if err != nil {
		return fmt.Errorf("创建SHEIN处理器失败: %w", err)
	}

	if err := serviceManager.RegisterProcessor("shein", sheinProcessor); err != nil {
		return fmt.Errorf("注册SHEIN处理器失败: %w", err)
	}

	r.logger.Info(" SHEIN处理器注册成功")
	return nil
}

// RegisterTemuProcessor 只注册 TEMU 平台处理器。
func (r *PlatformRegistry) RegisterTemuProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	if err := r.initForSinglePlatform(serviceManager, true); err != nil {
		return err
	}
	return r.registerTemuPlatform(ctx, serviceManager)
}

// RegisterSheinProcessor 只注册 SHEIN 平台处理器。
func (r *PlatformRegistry) RegisterSheinProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	if err := r.initForSinglePlatform(serviceManager, true); err != nil {
		return err
	}
	return r.registerSheinPlatform(ctx, serviceManager)
}

// RegisterAmazonProcessor 只注册 Amazon 平台处理器。
func (r *PlatformRegistry) RegisterAmazonProcessor(ctx context.Context, serviceManager *ServiceManager) error {
	if err := r.initForSinglePlatform(serviceManager, false); err != nil {
		return err
	}
	return r.registerAmazonPlatform(ctx, serviceManager)
}

// initForSinglePlatform 单平台注册前的公共初始化。
// needsAmazon 为 true 时同时初始化共享 Amazon 处理器（TEMU/SHEIN 需要）。
func (r *PlatformRegistry) initForSinglePlatform(serviceManager *ServiceManager, needsAmazon bool) error {
	r.rabbitmqClient = serviceManager.GetClient()
	if needsAmazon {
		return r.initializeSharedResources()
	}
	return r.initializeManagementClient()
}

// initializeManagementClient 初始化管理客户端并设置访问令牌。
// 与 initializeSharedResources 的区别：不创建共享的 Amazon 处理器。
func (r *PlatformRegistry) initializeManagementClient() error {
	r.managementClient = management.NewClientManager(&r.config.Management)
	return r.applyAccessToken()
}

// parsePlatformList 解析平台列表
func parsePlatformList(platformsStr string) []string {
	platforms := strings.Split(platformsStr, ",")
	result := make([]string, 0, len(platforms))

	for _, p := range platforms {
		trimmed := strings.TrimSpace(strings.ToLower(p))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// getEnabledPlatformsFromConfig 从配置文件中获取启用的平台列表
func getEnabledPlatformsFromConfig(cfg *config.Config) []string {
	platforms := make([]string, 0)

	// 检查Amazon配置（Amazon配置在单独的字段中）
	if cfg.Amazon.Enabled {
		platforms = append(platforms, "amazon")
	}

	// 检查其他平台配置
	if cfg.Platforms.Temu.Enabled {
		platforms = append(platforms, "temu")
	}

	if cfg.Platforms.Shein.Enabled {
		platforms = append(platforms, "shein")
	}

	if cfg.Platforms.Alibaba1688.Enabled {
		platforms = append(platforms, "alibaba1688")
	}

	return platforms
}

// containsPlatform 检查平台列表是否包含指定平台
func containsPlatform(platforms []string, platform string) bool {
	platform = strings.ToLower(platform)
	for _, p := range platforms {
		if strings.ToLower(p) == platform {
			return true
		}
	}
	return false
}

// GetSharedAmazonProcessor 获取共享的Amazon处理器
func (r *PlatformRegistry) GetSharedAmazonProcessor() *amazon.AmazonProcessor {
	return r.sharedAmazonProcessor
}
