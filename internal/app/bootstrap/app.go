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

type ApplicationBootstrap struct {
	logger           *logrus.Logger
	configManager    config.ConfigManager
	lifecycleManager lifecycle.LifecycleManager
	services         *appServices
	appVersion       string
}

func NewApplicationBootstrap(logger *logrus.Logger) *ApplicationBootstrap {
	return &ApplicationBootstrap{
		logger:           logger,
		configManager:    config.NewConfigManager(logger),
		lifecycleManager: lifecycle.NewLifecycleManager(logger),
	}
}

func (a *ApplicationBootstrap) Initialize(configPath, appVersion string) error {
	a.logger.Info("寮€濮嬪垵濮嬪寲搴旂敤...")
	a.appVersion = appVersion

	if err := a.loadConfiguration(configPath); err != nil {
		return fmt.Errorf("鍔犺浇閰嶇疆澶辫触: %w", err)
	}

	svc, err := buildServices(a.configManager.GetCurrent(), a.logger)
	if err != nil {
		return fmt.Errorf("鏋勫缓鏈嶅姟澶辫触: %w", err)
	}
	a.services = svc

	if err := registerComponents(a.lifecycleManager, a.services, a.logger, a.appVersion); err != nil {
		return fmt.Errorf("娉ㄥ唽缁勪欢澶辫触: %w", err)
	}

	a.logger.Info("鉁?搴旂敤鍒濆鍖栧畬鎴?")
	return nil
}

func (a *ApplicationBootstrap) Start(ctx context.Context, appVersion string) error {
	a.logger.Info("寮€濮嬪惎鍔ㄥ簲鐢?..")
	if err := a.lifecycleManager.StartAll(ctx); err != nil {
		return fmt.Errorf("鍚姩缁勪欢澶辫触: %w", err)
	}
	a.logger.Info("鉁?搴旂敤鍚姩瀹屾垚")
	return nil
}

func (a *ApplicationBootstrap) Stop(ctx context.Context) error {
	a.logger.Info("寮€濮嬪仠姝㈠簲鐢?..")
	if err := a.lifecycleManager.StopAll(ctx); err != nil {
		a.logger.Errorf("鍋滄缁勪欢澶辫触: %v", err)
	}
	a.logger.Info("鉁?搴旂敤鍋滄瀹屾垚")
	return nil
}

func (a *ApplicationBootstrap) GetConfigManager() config.ConfigManager {
	return a.configManager
}

func (a *ApplicationBootstrap) GetLifecycleManager() lifecycle.LifecycleManager {
	return a.lifecycleManager
}

func (a *ApplicationBootstrap) loadConfiguration(configPath string) error {
	a.logger.Infof("鍔犺浇閰嶇疆鏂囦欢: %s", configPath)
	source := config.NewFileConfigSource(configPath)
	cfg, err := a.configManager.Load(source)
	if err != nil {
		return err
	}
	a.logger.Infof("娴忚鍣ㄩ厤缃?- 鍚敤: %v, 璺緞: %s, 姹犲ぇ灏? %d",
		cfg.Browser.Enabled, cfg.Browser.BrowserPath, cfg.Browser.PoolSize)
	a.logger.Infof("绠＄悊绯荤粺閰嶇疆 - URL: %s, 瀹㈡埛绔疘D: %s",
		cfg.Management.BaseURL, cfg.Management.ClientID)
	return nil
}

func buildServices(cfg *config.Config, logger *logrus.Logger) (*appServices, error) {
	if cfg == nil {
		return nil, fmt.Errorf("閰嶇疆鏈姞杞?")
	}

	if err := InitializePrompts(context.Background(), cfg, logger); err != nil {
		logger.Warnf("Prompt 娉ㄥ唽琛ㄥ垵濮嬪寲澶辫触锛屽皢浣跨敤纭紪鐮?fallback: %v", err)
	}

	resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{
		NeedAmazonCrawler: true,
	})
	if err != nil {
		return nil, fmt.Errorf("鏋勫缓鍏变韩璧勬簮澶辫触: %w", err)
	}

	return &appServices{
		cfg:              cfg,
		authClient:       resources.AuthClient,
		managementClient: resources.ManagementClient,
		amazonCrawler:    resources.AmazonCrawler,
		processorService: runner.NewProcessorServiceWithDependencies(logger, resources.ManagementClient, resources.AmazonCrawler),
		schedulerService: runner.NewSchedulerServiceWithDependencies(
			logger,
			resources.ManagementClient,
			cfg,
			resources.AmazonCrawler,
			nil,
			BuildSchedulerDependencies(resources.ManagementClient, cfg, resources.AmazonCrawler, nil),
		),
	}, nil
}
