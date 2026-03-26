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
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func registerComponents(
	lm lifecycle.LifecycleManager,
	svc *appServices,
	logger *logrus.Logger,
	appVersion string,
) error {
	deps, err := registerCoreComponents(lm, svc, logger, appVersion)
	if err != nil {
		return err
	}

	if err := registerTaskFetcherComponent(lm, svc, logger, deps); err != nil {
		return err
	}

	return registerSchedulerComponent(lm, svc, logger, deps)
}

func registerCoreComponents(
	lm lifecycle.LifecycleManager,
	svc *appServices,
	logger *logrus.Logger,
	appVersion string,
) ([]string, error) {
	if err := lm.Register(newUpdaterComponent(logger, svc.cfg, appVersion)); err != nil {
		return nil, err
	}

	deps := []string{"updater"}

	if svc.rabbitmqClient != nil {
		if err := lm.Register(newRabbitMQClientComponent(svc.rabbitmqClient, logger)); err != nil {
			return nil, err
		}
		deps = append(deps, "rabbitmq-client")
	}

	if svc.cfg.Platforms.Temu.Enabled {
		temuProc, err := buildTemuProcessor(svc, logger)
		if err != nil {
			return nil, fmt.Errorf("build TEMU processor: %w", err)
		}
		svc.temuProcessor = temuProc
		if err := lm.Register(newTemuComponent(temuProc, logger)); err != nil {
			return nil, err
		}
		deps = append(deps, "temu-processor")
	}

	if svc.cfg.Platforms.Shein.Enabled {
		sheinProc, err := buildSheinProcessor(svc, logger)
		if err != nil {
			return nil, fmt.Errorf("build SHEIN processor: %w", err)
		}
		svc.sheinProcessor = sheinProc
		if err := lm.Register(newSheinComponent(sheinProc, logger)); err != nil {
			return nil, err
		}
		deps = append(deps, "shein-processor")
	}

	return deps, nil
}

func registerTaskFetcherComponent(
	lm lifecycle.LifecycleManager,
	svc *appServices,
	logger *logrus.Logger,
	deps []string,
) error {
	if !svc.cfg.Platforms.Temu.Enabled && !svc.cfg.Platforms.Shein.Enabled {
		return nil
	}

	return lm.Register(newTaskFetcherComponent(svc.processorService, svc.authClient, svc.cfg, logger, deps))
}

func registerSchedulerComponent(
	lm lifecycle.LifecycleManager,
	svc *appServices,
	logger *logrus.Logger,
	deps []string,
) error {
	return lm.Register(newSchedulerComponent(svc.schedulerService, logger, deps))
}

func newUpdaterComponent(logger *logrus.Logger, cfg *config.Config, appVersion string) *updaterComponent {
	return &updaterComponent{
		BaseComponent: lifecycle.NewBaseComponent("updater", nil, 10),
		logger:        logger,
		cfg:           cfg,
		appVersion:    appVersion,
	}
}

type updaterComponent struct {
	*lifecycle.BaseComponent
	logger     *logrus.Logger
	cfg        *config.Config
	appVersion string
}

func newRabbitMQClientComponent(client *rabbitmq.Client, logger *logrus.Logger) *rabbitmqClientComponent {
	return &rabbitmqClientComponent{
		BaseComponent: lifecycle.NewBaseComponent("rabbitmq-client", []string{"updater"}, 15),
		client:        client,
		logger:        logger,
	}
}

type rabbitmqClientComponent struct {
	*lifecycle.BaseComponent
	client *rabbitmq.Client
	logger *logrus.Logger
}

func (r *rabbitmqClientComponent) Start(ctx context.Context) error {
	if r.client == nil {
		return nil
	}
	if err := r.client.GetConnectionManager().Connect(ctx); err != nil {
		return fmt.Errorf("start RabbitMQ client: %w", err)
	}
	r.logger.Info("RabbitMQ client started for distributed crawler access")
	r.SetRunning(true)
	return nil
}

func (r *rabbitmqClientComponent) Stop(_ context.Context) error {
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			r.logger.Errorf("stop RabbitMQ client: %v", err)
		}
	}
	r.SetRunning(false)
	return nil
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
		u.logger.Infof("updater started: version=%s interval=%v", u.appVersion, interval)
	}
	u.SetRunning(true)
	return nil
}

func (u *updaterComponent) Stop(_ context.Context) error {
	u.SetRunning(false)
	return nil
}

func newTemuComponent(processor *temu.TemuProcessor, logger *logrus.Logger) *temuComponent {
	return &temuComponent{
		BaseComponent: lifecycle.NewBaseComponent("temu-processor", []string{"updater"}, 20),
		processor:     processor,
		logger:        logger,
	}
}

type temuComponent struct {
	*lifecycle.BaseComponent
	processor *temu.TemuProcessor
	logger    *logrus.Logger
}

func (t *temuComponent) Start(ctx context.Context) error {
	if err := t.processor.Start(ctx); err != nil {
		return fmt.Errorf("start TEMU processor: %w", err)
	}
	t.SetRunning(true)
	return nil
}

func (t *temuComponent) Stop(ctx context.Context) error {
	t.processor.Close(ctx)
	t.SetRunning(false)
	return nil
}

func newSheinComponent(processor *pipeline.SheinProcessor, logger *logrus.Logger) *sheinComponent {
	return &sheinComponent{
		BaseComponent: lifecycle.NewBaseComponent("shein-processor", []string{"updater"}, 20),
		processor:     processor,
		logger:        logger,
	}
}

type sheinComponent struct {
	*lifecycle.BaseComponent
	processor *pipeline.SheinProcessor
	logger    *logrus.Logger
}

func (s *sheinComponent) Start(ctx context.Context) error {
	if err := s.processor.Start(ctx); err != nil {
		return fmt.Errorf("start SHEIN processor: %w", err)
	}
	s.SetRunning(true)
	return nil
}

func (s *sheinComponent) Stop(ctx context.Context) error {
	s.processor.Close(ctx)
	s.SetRunning(false)
	return nil
}

func newTaskFetcherComponent(processorService runner.ProcessorService, authClient *auth.ClientCredentialsAuthClient, cfg *config.Config, logger *logrus.Logger, deps []string) *taskFetcherComponent {
	return &taskFetcherComponent{
		BaseComponent:    lifecycle.NewBaseComponent("task-fetcher", deps, 25),
		processorService: processorService,
		authClient:       authClient,
		cfg:              cfg,
		logger:           logger,
	}
}

type taskFetcherComponent struct {
	*lifecycle.BaseComponent
	processorService runner.ProcessorService
	authClient       *auth.ClientCredentialsAuthClient
	cfg              *config.Config
	logger           *logrus.Logger
}

func (t *taskFetcherComponent) Start(ctx context.Context) error {
	if err := t.processorService.StartProcessors(ctx, t.cfg, t.authClient); err != nil {
		return fmt.Errorf("start task fetcher: %w", err)
	}
	t.SetRunning(true)
	return nil
}

func (t *taskFetcherComponent) Stop(_ context.Context) error {
	if err := t.processorService.StopProcessors(); err != nil {
		t.logger.Errorf("stop task fetcher: %v", err)
	}
	t.SetRunning(false)
	return nil
}

func newSchedulerComponent(schedulerService runner.SchedulerService, logger *logrus.Logger, deps []string) *schedulerComponent {
	return &schedulerComponent{
		BaseComponent:    lifecycle.NewBaseComponent("scheduler", deps, 30),
		schedulerService: schedulerService,
		logger:           logger,
	}
}

type schedulerComponent struct {
	*lifecycle.BaseComponent
	schedulerService runner.SchedulerService
	logger           *logrus.Logger
}

func (s *schedulerComponent) Start(ctx context.Context) error {
	if err := s.schedulerService.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler service: %w", err)
	}
	s.SetRunning(true)
	return nil
}

func (s *schedulerComponent) Stop(ctx context.Context) error {
	if err := s.schedulerService.Stop(ctx); err != nil {
		s.logger.Errorf("stop scheduler service: %v", err)
	}
	s.SetRunning(false)
	return nil
}
