// Package bootstrap 提供组件适配器实现
package bootstrap

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/app/runner"
	"task-processor/internal/app/updater"
	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/auth"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

// registerComponents 注册所有组件到生命周期管理器
func registerComponents(
	lm lifecycle.LifecycleManager,
	svc *appServices,
	logger *logrus.Logger,
	appVersion string,
) error {
	// 更新服务（无依赖，优先级最高）
	if err := lm.Register(&updaterComponent{
		BaseComponent: lifecycle.NewBaseComponent("updater", nil, 10),
		logger:        logger,
		cfg:           svc.cfg,
		appVersion:    appVersion,
	}); err != nil {
		return err
	}

	deps := []string{"updater"}

	// TEMU 处理器
	if svc.cfg.Platforms.Temu.Enabled {
		temuProc, err := buildTemuProcessor(svc, logger)
		if err != nil {
			return fmt.Errorf("构建TEMU处理器失败: %w", err)
		}
		svc.temuProcessor = temuProc
		if err := lm.Register(&temuComponent{
			BaseComponent: lifecycle.NewBaseComponent("temu-processor", []string{"updater"}, 20),
			processor:     temuProc,
			logger:        logger,
		}); err != nil {
			return err
		}
		deps = append(deps, "temu-processor")
	}

	// SHEIN 处理器
	if svc.cfg.Platforms.Shein.Enabled {
		sheinProc, err := buildSheinProcessor(svc, logger)
		if err != nil {
			return fmt.Errorf("构建SHEIN处理器失败: %w", err)
		}
		svc.sheinProcessor = sheinProc
		if err := lm.Register(&sheinComponent{
			BaseComponent: lifecycle.NewBaseComponent("shein-processor", []string{"updater"}, 20),
			processor:     sheinProc,
			logger:        logger,
		}); err != nil {
			return err
		}
		deps = append(deps, "shein-processor")
	}

	// 任务获取器（依赖处理器）
	if svc.cfg.Platforms.Temu.Enabled || svc.cfg.Platforms.Shein.Enabled {
		if err := lm.Register(&taskFetcherComponent{
			BaseComponent:    lifecycle.NewBaseComponent("task-fetcher", deps, 25),
			processorService: svc.processorService,
			authClient:       svc.authClient,
			cfg:              svc.cfg,
			logger:           logger,
		}); err != nil {
			return err
		}
	}

	// 调度服务
	return lm.Register(&schedulerComponent{
		BaseComponent:    lifecycle.NewBaseComponent("scheduler", deps, 30),
		schedulerService: svc.schedulerService,
		logger:           logger,
	})
}

// updaterComponent 更新服务组件
type updaterComponent struct {
	*lifecycle.BaseComponent
	logger     *logrus.Logger
	cfg        *config.Config
	appVersion string
}

func (u *updaterComponent) Start(_ context.Context) error {
	if u.cfg.Updater.Enabled {
		updateURL := u.cfg.Updater.UpdateURL
		if updateURL == "" {
			updateURL = "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json"
		}
		interval := time.Duration(u.cfg.Updater.CheckInterval) * time.Second
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		go updater.NewUpdater(u.appVersion, updateURL, interval, u.cfg.Updater.InsecureSkipVerify).Start()
		u.logger.Infof("自动更新器已启动 (版本: %s, 间隔: %v)", u.appVersion, interval)
	}
	u.SetRunning(true)
	return nil
}

func (u *updaterComponent) Stop(_ context.Context) error {
	u.SetRunning(false)
	return nil
}

// temuComponent TEMU 处理器组件
type temuComponent struct {
	*lifecycle.BaseComponent
	processor *temu.TemuProcessor
	logger    *logrus.Logger
}

func (t *temuComponent) Start(ctx context.Context) error {
	if err := t.processor.Start(ctx); err != nil {
		return fmt.Errorf("启动TEMU处理器失败: %w", err)
	}
	t.SetRunning(true)
	return nil
}

func (t *temuComponent) Stop(ctx context.Context) error {
	t.processor.Close(ctx)
	t.SetRunning(false)
	return nil
}

// sheinComponent SHEIN 处理器组件
type sheinComponent struct {
	*lifecycle.BaseComponent
	processor *pipeline.SheinProcessor
	logger    *logrus.Logger
}

func (s *sheinComponent) Start(ctx context.Context) error {
	if err := s.processor.Start(ctx); err != nil {
		return fmt.Errorf("启动SHEIN处理器失败: %w", err)
	}
	s.SetRunning(true)
	return nil
}

func (s *sheinComponent) Stop(ctx context.Context) error {
	s.processor.Close(ctx)
	s.SetRunning(false)
	return nil
}

// taskFetcherComponent 任务获取器组件
type taskFetcherComponent struct {
	*lifecycle.BaseComponent
	processorService runner.ProcessorService
	authClient       *auth.ClientCredentialsAuthClient
	cfg              *config.Config
	logger           *logrus.Logger
}

func (t *taskFetcherComponent) Start(ctx context.Context) error {
	if err := t.processorService.StartProcessors(ctx, t.cfg, t.authClient); err != nil {
		return fmt.Errorf("启动任务获取器失败: %w", err)
	}
	t.SetRunning(true)
	return nil
}

func (t *taskFetcherComponent) Stop(_ context.Context) error {
	if err := t.processorService.StopProcessors(); err != nil {
		t.logger.Errorf("停止任务获取器失败: %v", err)
	}
	t.SetRunning(false)
	return nil
}

// schedulerComponent 调度服务组件
type schedulerComponent struct {
	*lifecycle.BaseComponent
	schedulerService runner.SchedulerService
	logger           *logrus.Logger
}

func (s *schedulerComponent) Start(ctx context.Context) error {
	if err := s.schedulerService.Start(ctx); err != nil {
		return fmt.Errorf("启动调度服务失败: %w", err)
	}
	s.SetRunning(true)
	return nil
}

func (s *schedulerComponent) Stop(ctx context.Context) error {
	if err := s.schedulerService.Stop(ctx); err != nil {
		s.logger.Errorf("停止调度服务失败: %v", err)
	}
	s.SetRunning(false)
	return nil
}
