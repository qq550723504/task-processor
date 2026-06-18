package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/database"
	"task-processor/internal/productenrich"
)

func newLLMManager(cfg config.OpenAIConfig) (productenrich.LLMManager, error) {
	manager, err := newOpenAIManager(cfg)
	if err != nil {
		return nil, err
	}
	return productenrich.NewLLMManagerAdapterFromManager(manager)
}

func newOpenAIManager(cfg config.OpenAIConfig) (*openaiclient.Manager, error) {
	return openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.ToClientConfigs(),
		DefaultClient: "default",
	})
}

func newDBOpenAICredentialResolver(cfg *config.DatabaseConfig, logger *logrus.Logger) (*openaiclient.GormCredentialResolver, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if shouldAutoMigrateProductListingAPIRuntime() {
		if err := db.AutoMigrate(&openaiclient.AIClientCredential{}); err != nil {
			return nil, nil, fmt.Errorf("openai credential auto-migrate failed: %w", err)
		}
	}
	resolver := openaiclient.NewGormCredentialResolver(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return resolver, closer, nil
}
