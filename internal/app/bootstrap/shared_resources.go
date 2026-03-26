package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/prompt"

	"github.com/sirupsen/logrus"
)

// SharedResourceOptions controls which shared runtime dependencies are built.
type SharedResourceOptions struct {
	NeedAmazonCrawler bool
}

// SharedResources groups dependencies that were previously assembled in multiple places.
type SharedResources struct {
	AuthClient       *auth.ClientCredentialsAuthClient
	ManagementClient *management.ClientManager
	AmazonCrawler    *amazon.AmazonProcessor
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

	runtime, err := buildManagementRuntime(cfg, logger)
	if err != nil {
		return nil, err
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
		resources.AmazonCrawler = buildAmazonCrawler(cfg, logger)
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

	managementClient := management.NewClientManager(&cfg.Management)
	managementClient.GetClient()
	managementClient.SetUserToken(accessToken, tenantID)
	managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)

	return &managementRuntime{
		authClient:       authClient,
		managementClient: managementClient,
	}, nil
}

func buildAmazonCrawler(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor {
	return amazon.CreateProcessor(cfg, logger)
}

func resolveTenantID(cfg *config.Config) string {
	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		return "1"
	}

	return tenantID
}
