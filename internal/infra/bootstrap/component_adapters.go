// Package bootstrap 提供组件适配器实现
package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/app/service"
	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/di"
	"task-processor/internal/platforms/shein/service/pipeline"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// ComponentAdapters 组件适配器管理器
type ComponentAdapters struct {
	container di.Container
	logger    *logrus.Logger
}

// NewComponentAdapters 创建组件适配器管理器
func NewComponentAdapters(container di.Container, logger *logrus.Logger) *ComponentAdapters {
	return &ComponentAdapters{
		container: container,
		logger:    logger,
	}
}

// RegisterAllComponents 注册所有组件到生命周期管理器
func (c *ComponentAdapters) RegisterAllComponents(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config) error {
	// 注册更新服务组件
	if err := c.registerUpdaterComponent(lifecycleManager, cfg); err != nil {
		return err
	}

	// 注册平台处理器组件
	if err := c.registerProcessorComponents(lifecycleManager, cfg); err != nil {
		return err
	}

	// 注册调度服务组件
	if err := c.registerSchedulerComponent(lifecycleManager, cfg); err != nil {
		return err
	}

	return nil
}

// registerUpdaterComponent 注册更新服务组件
func (c *ComponentAdapters) registerUpdaterComponent(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config) error {
	// 总是注册更新服务组件，但根据配置决定是否启动自动更新功能
	c.logger.Debug("注册更新服务组件...")

	component := &UpdaterServiceComponent{
		BaseComponent: lifecycle.NewBaseComponent("updater", []string{}, 10),
		container:     c.container,
		logger:        c.logger,
		config:        cfg,
	}

	return lifecycleManager.Register(component)
}

// registerProcessorComponents 注册处理器组件
func (c *ComponentAdapters) registerProcessorComponents(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config) error {
	// 注册TEMU处理器组件
	if cfg.Platforms.Temu.Enabled {
		component := &TemuProcessorComponent{
			BaseComponent: lifecycle.NewBaseComponent("temu-processor", []string{"updater"}, 20),
			container:     c.container,
			logger:        c.logger,
		}
		if err := lifecycleManager.Register(component); err != nil {
			return err
		}
	}

	// 注册SHEIN处理器组件
	if cfg.Platforms.Shein.Enabled {
		component := &SheinProcessorComponent{
			BaseComponent: lifecycle.NewBaseComponent("shein-processor", []string{"updater"}, 20),
			container:     c.container,
			logger:        c.logger,
		}
		if err := lifecycleManager.Register(component); err != nil {
			return err
		}
	}

	return nil
}

// registerSchedulerComponent 注册调度服务组件
func (c *ComponentAdapters) registerSchedulerComponent(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config) error {
	// 构建依赖列表
	dependencies := []string{"updater"}
	if cfg.Platforms.Temu.Enabled {
		dependencies = append(dependencies, "temu-processor")
	}
	if cfg.Platforms.Shein.Enabled {
		dependencies = append(dependencies, "shein-processor")
	}

	component := &SchedulerServiceComponent{
		BaseComponent: lifecycle.NewBaseComponent("scheduler", dependencies, 30),
		container:     c.container,
		logger:        c.logger,
		config:        cfg,
	}

	return lifecycleManager.Register(component)
}

// UpdaterServiceComponent 更新服务组件适配器
type UpdaterServiceComponent struct {
	*lifecycle.BaseComponent
	container di.Container
	logger    *logrus.Logger
	config    *config.Config
}

// Start 启动更新服务
func (u *UpdaterServiceComponent) Start(ctx context.Context) error {
	u.logger.Info("启动更新服务组件...")

	updaterService, err := u.container.Get("updaterService")
	if err != nil {
		return fmt.Errorf("获取更新服务失败: %w", err)
	}

	// 根据配置决定是否启动自动更新器
	if u.config.Updater.Enabled {
		// 启动自动更新器（保持原有逻辑）
		updaterService.(*service.UpdaterService).StartAutoUpdater(u.config, "")
		u.logger.Info("自动更新器已启动")
	} else {
		u.logger.Info("自动更新功能已禁用")
	}

	u.SetRunning(true)
	u.logger.Info("✅ 更新服务组件启动成功")
	return nil
}

// Stop 停止更新服务
func (u *UpdaterServiceComponent) Stop(ctx context.Context) error {
	u.logger.Info("停止更新服务组件...")
	u.SetRunning(false)
	u.logger.Info("✅ 更新服务组件停止成功")
	return nil
}

// TemuProcessorComponent TEMU处理器组件适配器
type TemuProcessorComponent struct {
	*lifecycle.BaseComponent
	container di.Container
	logger    *logrus.Logger
}

// Start 启动TEMU处理器
func (t *TemuProcessorComponent) Start(ctx context.Context) error {
	t.logger.Info("启动TEMU处理器组件...")

	processor, err := t.container.Get("temuProcessor")
	if err != nil {
		return fmt.Errorf("获取TEMU处理器失败: %w", err)
	}

	// 启动处理器（保持原有逻辑）
	temuProcessor := processor.(*temu.TemuProcessor)
	if err := temuProcessor.Start(ctx); err != nil {
		return fmt.Errorf("启动TEMU处理器失败: %w", err)
	}

	t.SetRunning(true)
	t.logger.Info("✅ TEMU处理器组件启动成功")
	return nil
}

// Stop 停止TEMU处理器
func (t *TemuProcessorComponent) Stop(ctx context.Context) error {
	t.logger.Info("停止TEMU处理器组件...")

	processor, err := t.container.Get("temuProcessor")
	if err != nil {
		t.logger.Errorf("获取TEMU处理器失败: %v", err)
		t.SetRunning(false)
		return nil
	}

	// 停止处理器（保持原有逻辑）
	temuProcessor := processor.(*temu.TemuProcessor)
	temuProcessor.Close(ctx)

	t.SetRunning(false)
	t.logger.Info("✅ TEMU处理器组件停止成功")
	return nil
}

// SheinProcessorComponent SHEIN处理器组件适配器
type SheinProcessorComponent struct {
	*lifecycle.BaseComponent
	container di.Container
	logger    *logrus.Logger
}

// Start 启动SHEIN处理器
func (s *SheinProcessorComponent) Start(ctx context.Context) error {
	s.logger.Info("启动SHEIN处理器组件...")

	processor, err := s.container.Get("sheinProcessor")
	if err != nil {
		return fmt.Errorf("获取SHEIN处理器失败: %w", err)
	}

	// 启动处理器（保持原有逻辑）
	sheinProcessor := processor.(*pipeline.SheinProcessor)
	if err := sheinProcessor.Start(ctx); err != nil {
		return fmt.Errorf("启动SHEIN处理器失败: %w", err)
	}

	s.SetRunning(true)
	s.logger.Info("✅ SHEIN处理器组件启动成功")
	return nil
}

// Stop 停止SHEIN处理器
func (s *SheinProcessorComponent) Stop(ctx context.Context) error {
	s.logger.Info("停止SHEIN处理器组件...")

	processor, err := s.container.Get("sheinProcessor")
	if err != nil {
		s.logger.Errorf("获取SHEIN处理器失败: %v", err)
		s.SetRunning(false)
		return nil
	}

	// 停止处理器（保持原有逻辑）
	sheinProcessor := processor.(*pipeline.SheinProcessor)
	sheinProcessor.Close(ctx)

	s.SetRunning(false)
	s.logger.Info("✅ SHEIN处理器组件停止成功")
	return nil
}

// SchedulerServiceComponent 调度服务组件适配器
type SchedulerServiceComponent struct {
	*lifecycle.BaseComponent
	container di.Container
	logger    *logrus.Logger
	config    *config.Config
}

// Start 启动调度服务
func (s *SchedulerServiceComponent) Start(ctx context.Context) error {
	s.logger.Info("启动调度服务组件...")

	schedulerService, err := s.container.Get("schedulerService")
	if err != nil {
		return fmt.Errorf("获取调度服务失败: %w", err)
	}

	// 启动调度服务（保持原有逻辑）
	if err := schedulerService.(service.SchedulerService).Start(ctx); err != nil {
		return fmt.Errorf("启动调度服务失败: %w", err)
	}

	s.SetRunning(true)
	s.logger.Info("✅ 调度服务组件启动成功")
	return nil
}

// Stop 停止调度服务
func (s *SchedulerServiceComponent) Stop(ctx context.Context) error {
	s.logger.Info("停止调度服务组件...")

	schedulerService, err := s.container.Get("schedulerService")
	if err != nil {
		s.logger.Errorf("获取调度服务失败: %v", err)
		s.SetRunning(false)
		return nil
	}

	// 停止调度服务（保持原有逻辑）
	if err := schedulerService.(service.SchedulerService).Stop(ctx); err != nil {
		s.logger.Errorf("停止调度服务失败: %v", err)
	}

	s.SetRunning(false)
	s.logger.Info("✅ 调度服务组件停止成功")
	return nil
}
