// Package messaging 提供多平台处理器注册功能
package messaging

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/pkg/management"
	platformAmazon "task-processor/internal/platforms/amazon"
	"task-processor/internal/platforms/shein/service/pipeline"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// PlatformRegistry 多平台处理器注册器
type PlatformRegistry struct {
	config                *config.Config
	logger                *logrus.Logger
	managementClient      *management.ClientManager
	sharedAmazonProcessor *amazon.AmazonProcessor
	enabledPlatforms      []string
}

// NewPlatformRegistry 创建平台注册器
func NewPlatformRegistry(
	cfg *config.Config,
	logger *logrus.Logger,
	platformsStr string,
) *PlatformRegistry {
	// 解析平台列表
	enabledPlatforms := parsePlatformList(platformsStr)
	logger.Infof("🔧 启用的平台: %v", enabledPlatforms)

	return &PlatformRegistry{
		config:           cfg,
		logger:           logger,
		enabledPlatforms: enabledPlatforms,
	}
}

// RegisterAllProcessors 注册所有启用的平台处理器
func (r *PlatformRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager) error {
	r.logger.Info("📦 开始注册平台处理器...")

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

	r.logger.Info("✅ 所有平台处理器注册完成")
	return nil
}

// initializeSharedResources 初始化共享资源
func (r *PlatformRegistry) initializeSharedResources() error {
	r.logger.Info("🔧 初始化共享资源...")

	// 创建管理客户端（所有平台共享）
	r.managementClient = management.NewClientManager(&r.config.Management)

	// 创建共享的Amazon处理器（TEMU和SHEIN需要）
	if r.needsAmazonProcessor() {
		r.sharedAmazonProcessor = amazon.CreateProcessor(r.config, r.logger)
	}

	r.logger.Info("✅ 共享资源初始化完成")
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

	r.logger.Info("📦 注册Amazon平台处理器...")

	// 使用完整的Amazon平台处理器（用于上架）
	amazonProcessor := platformAmazon.NewProcessor(ctx, r.config, r.logger)

	if err := serviceManager.RegisterProcessor("amazon", amazonProcessor); err != nil {
		return fmt.Errorf("注册Amazon平台处理器失败: %w", err)
	}

	r.logger.Info("✅ Amazon平台处理器注册成功")
	return nil
}

// registerTemuPlatform 注册TEMU平台处理器
func (r *PlatformRegistry) registerTemuPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "temu") {
		r.logger.Debug("跳过TEMU平台注册")
		return nil
	}

	r.logger.Info("📦 注册TEMU处理器...")

	temuProcessor, err := temu.NewTemuProcessor(
		ctx,
		r.config,
		r.logger,
		r.managementClient,
		r.sharedAmazonProcessor,
	)
	if err != nil {
		return fmt.Errorf("创建TEMU处理器失败: %w", err)
	}

	if err := serviceManager.RegisterProcessor("temu", temuProcessor); err != nil {
		return fmt.Errorf("注册TEMU处理器失败: %w", err)
	}

	r.logger.Info("✅ TEMU处理器注册成功")
	return nil
}

// registerSheinPlatform 注册SHEIN平台处理器
func (r *PlatformRegistry) registerSheinPlatform(ctx context.Context, serviceManager *ServiceManager) error {
	if !containsPlatform(r.enabledPlatforms, "shein") {
		r.logger.Debug("跳过SHEIN平台注册")
		return nil
	}

	r.logger.Info("📦 注册SHEIN处理器...")

	sheinProcessor, err := pipeline.NewSheinProcessor(
		ctx,
		r.config,
		r.logger,
		r.managementClient,
		r.sharedAmazonProcessor,
	)
	if err != nil {
		return fmt.Errorf("创建SHEIN处理器失败: %w", err)
	}

	if err := serviceManager.RegisterProcessor("shein", sheinProcessor); err != nil {
		return fmt.Errorf("注册SHEIN处理器失败: %w", err)
	}

	r.logger.Info("✅ SHEIN处理器注册成功")
	return nil
}

// parsePlatformList 解析平台列表
func parsePlatformList(platformsStr string) []string {
	if platformsStr == "" {
		return []string{}
	}

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
