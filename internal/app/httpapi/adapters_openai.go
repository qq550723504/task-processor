package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
	platformdatabase "task-processor/internal/platform/database"
)

func newOpenAIManager(cfg config.OpenAIConfig, logger *logrus.Entry) (*openaiclient.Manager, error) {
	return openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.ToClientConfigs(),
		DefaultClient: "default",
		Logger:        openaiclient.AdaptLogrus(logger),
	})
}

func newDBOpenAICredentialResolver(cfg *config.DatabaseConfig, logger *logrus.Logger) (*openaiclient.GormCredentialResolver, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	databaseConfig := configadapter.Database(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	resolver := openaiclient.NewGormCredentialResolver(db)
	closer := func() error { return platformdatabase.CloseShared(databaseConfig, db) }
	return resolver, closer, nil
}
