// Package main 提供测试初始化功能
package main

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/auth"
	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// TestInitializer 测试初始化器
type TestInitializer struct {
	ctx             context.Context
	cfg             *config.Config
	logger          *logrus.Entry
	mgmtClient      *management.ClientManager
	amazonProcessor *amazon.AmazonProcessor
}

// NewTestInitializer 创建测试初始化器
func NewTestInitializer(ctx context.Context, cfg *config.Config, logger *logrus.Entry) *TestInitializer {
	return &TestInitializer{
		ctx:    ctx,
		cfg:    cfg,
		logger: logger,
	}
}

// Initialize 初始化所有依赖
func (ti *TestInitializer) Initialize() error {
	ti.logger.Info("🔄 开始初始化测试依赖...")

	// 1. 初始化管理客户端
	if err := ti.initializeManagementClient(); err != nil {
		return fmt.Errorf("初始化管理客户端失败: %w", err)
	}

	// 2. 初始化Amazon处理器
	if err := ti.initializeAmazonProcessor(); err != nil {
		return fmt.Errorf("初始化Amazon处理器失败: %w", err)
	}

	ti.logger.Info("✅ 测试依赖初始化完成")
	return nil
}

// initializeManagementClient 初始化管理客户端
func (ti *TestInitializer) initializeManagementClient() error {
	ti.logger.Info("初始化管理客户端...")

	// 创建认证客户端
	authClient := auth.NewClientCredentialsAuthClient(
		ti.cfg.Management.BaseURL,
		ti.cfg.Management.ClientID,
		ti.cfg.Management.ClientSecret,
		ti.cfg.Management.TenantID,
		ti.logger.Logger,
	)

	// 创建管理客户端
	mgmtClient := management.NewClientManager(&ti.cfg.Management)

	// 获取访问令牌
	token, err := authClient.GetAccessToken()
	if err != nil {
		ti.logger.WithError(err).Error("获取访问令牌失败")
		return fmt.Errorf("认证失败，无法获取访问令牌: %w", err)
	}

	// 设置访问令牌
	mgmtClient.SetUserToken(token, ti.cfg.Management.TenantID)
	ti.logger.Info("访问令牌设置成功")

	// 设置数据新鲜度
	mgmtClient.SetDataFreshnessDays(ti.cfg.Amazon.DataFreshnessDays)

	ti.mgmtClient = mgmtClient
	ti.logger.Info("✅ 管理客户端初始化完成")
	return nil
}

// initializeAmazonProcessor 初始化Amazon处理器
func (ti *TestInitializer) initializeAmazonProcessor() error {
	if !ti.cfg.Amazon.Enabled {
		ti.logger.Info("Amazon处理器未启用，跳过初始化")
		return nil
	}

	ti.logger.Info("初始化Amazon处理器...")

	// 创建Amazon处理器
	amazonProcessor := amazon.NewAmazonProcessor(ti.cfg)
	if amazonProcessor == nil {
		return fmt.Errorf("创建Amazon处理器失败")
	}

	ti.amazonProcessor = amazonProcessor
	ti.logger.Info("✅ Amazon处理器初始化完成")
	return nil
}

// GetManagementClient 获取管理客户端
func (ti *TestInitializer) GetManagementClient() *management.ClientManager {
	return ti.mgmtClient
}

// GetAmazonProcessor 获取Amazon处理器
func (ti *TestInitializer) GetAmazonProcessor() *amazon.AmazonProcessor {
	return ti.amazonProcessor
}

// Cleanup 清理资源
func (ti *TestInitializer) Cleanup() {
	ti.logger.Info("🧹 开始清理测试资源...")

	if ti.amazonProcessor != nil {
		ti.amazonProcessor.Shutdown()
		ti.logger.Info("Amazon处理器已关闭")
	}

	ti.logger.Info("✅ 测试资源清理完成")
}
