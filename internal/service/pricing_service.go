// Package service 提供核价服务功能
package service

import (
	"context"

	"task-processor/internal/common/management"
	"task-processor/internal/platforms/temu/handlers"

	"github.com/sirupsen/logrus"
)

// PricingService 核价服务接口
type PricingService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() map[string]any
}

// pricingServiceImpl 核价服务实现
type pricingServiceImpl struct {
	logger                 *logrus.Logger
	managementClient       *management.ClientManager
	temuAutoPricingHandler *handlers.AutoPricingHandler
	ctx                    context.Context
	cancel                 context.CancelFunc
	running                bool
}

// NewPricingService 创建核价服务
func NewPricingService(logger *logrus.Logger) PricingService {
	return &pricingServiceImpl{
		logger: logger,
	}
}

// Start 启动核价服务
func (s *pricingServiceImpl) Start(ctx context.Context) error {
	if s.running {
		return nil
	}

	s.logger.Info("🚀 开始启动核价服务...")

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(ctx)

	// 获取共享资源
	if err := s.initializePricingResources(); err != nil {
		return err
	}

	// 启动各平台核价处理器
	if err := s.startPricingHandlers(); err != nil {
		return err
	}

	s.running = true

	return nil
}

// Stop 停止核价服务
func (s *pricingServiceImpl) Stop(ctx context.Context) error {
	if !s.running {
		return nil
	}

	s.logger.Info("🛑 开始停止核价服务...")

	// 停止TEMU核价处理器
	if s.temuAutoPricingHandler != nil {
		s.temuAutoPricingHandler.Stop()
		s.logger.Info("✅ TEMU核价处理器已停止")
	}

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("✅ 核价服务已停止")

	return nil
}

// GetStatus 获取核价服务状态
func (s *pricingServiceImpl) GetStatus() map[string]any {
	return map[string]any{
		"running": s.running,
		"handlers": map[string]any{
			"temu": s.temuAutoPricingHandler != nil,
		},
	}
}
