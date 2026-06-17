package resources

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	crawleramazon "task-processor/internal/integration/crawler/amazon"
	"task-processor/internal/prompt"

	"github.com/sirupsen/logrus"
)

// SharedResourceOptions controls which shared runtime dependencies are built.
type SharedResourceOptions struct {
	NeedAmazonCrawler          bool
	AllowMissingManagementAuth bool
	SkipManagementAuth         bool
}

// SharedResources groups dependencies that were previously assembled in multiple places.
type SharedResources struct {
	AuthClient       *auth.ClientCredentialsAuthClient
	ManagementClient *management.ClientManager
	AmazonCrawler    runner.CrawlSource
	RabbitMQClient   *rabbitmq.Client
}

type managementRuntime struct {
	authClient       *auth.ClientCredentialsAuthClient
	managementClient *management.ClientManager
}

// InitializePrompts centralizes prompt registry initialization.
func InitializePrompts(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}

	if err := prompt.InitGlobal(ctx, promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		return fmt.Errorf("initialize prompts: %w", err)
	}

	return nil
}

// BuildSharedResources centralizes auth, management client, and optional crawler assembly.
func BuildSharedResources(cfg *config.Config, logger *logrus.Logger, options SharedResourceOptions) (*SharedResources, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	runtime := &managementRuntime{
		managementClient: newConfiguredManagementClient(cfg, logger),
	}
	if !options.SkipManagementAuth {
		var err error
		runtime, err = buildManagementRuntime(cfg, logger)
		if err != nil {
			if !options.AllowMissingManagementAuth {
				return nil, err
			}

			logger.WithError(err).Warn("management runtime unavailable, continuing without management client")
			runtime = &managementRuntime{
				managementClient: newConfiguredManagementClient(cfg, logger),
			}
		}
	}

	resources := &SharedResources{
		AuthClient:       runtime.authClient,
		ManagementClient: runtime.managementClient,
	}

	if cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled {
		connManager := rabbitmq.NewConnectionManager(rabbitmq.ConnectionConfig{
			URL:               cfg.RabbitMQ.URL,
			ReconnectInterval: cfg.RabbitMQ.ReconnectInterval,
			MaxReconnectTries: cfg.RabbitMQ.MaxReconnectTries,
		}, logger)
		resources.RabbitMQClient = rabbitmq.NewClient(connManager, logger)
	}

	if options.NeedAmazonCrawler {
		resources.AmazonCrawler = BuildAmazonCrawler(cfg, logger)
	}

	return resources, nil
}

func buildManagementRuntime(cfg *config.Config, logger *logrus.Logger) (*managementRuntime, error) {
	tenantID := resolveTenantID(cfg)

	authClient := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		tenantID,
		logger,
	)

	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	managementClient := newConfiguredManagementClient(cfg, logger)
	managementClient.GetClient()
	managementClient.SetUserToken(accessToken, tenantID)

	return &managementRuntime{
		authClient:       authClient,
		managementClient: managementClient,
	}, nil
}

func newConfiguredManagementClient(cfg *config.Config, logger *logrus.Logger) *management.ClientManager {
	managementClient := management.NewClientManager(&cfg.Management)
	managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)

	if provider, err := management.NewLocalDataProvider(cfg.Database, cfg.Redis); err != nil {
		logger.WithError(err).Warn("failed to configure local management data provider")
	} else if provider != nil {
		managementClient.SetLocalDataProvider(provider)
	}

	cookieRedis := cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(cookieRedis.Host) != "" {
		if err := managementClient.SetSheinCookieRedisConfig(&cookieRedis); err != nil {
			logger.WithError(err).Warn("failed to configure SHEIN cookie Redis provider")
		}
	}

	return managementClient
}

// BuildAmazonCrawler constructs the concrete Amazon crawler at the bootstrap edge.
func BuildAmazonCrawler(cfg *config.Config, logger *logrus.Logger) runner.CrawlSource {
	return crawleramazon.NewLegacyCrawlSource(cfg, logger)
}

func resolveTenantID(cfg *config.Config) string {
	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		return "1"
	}

	return tenantID
}
