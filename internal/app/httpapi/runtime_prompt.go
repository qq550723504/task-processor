package httpapi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/prompt"
)

func initPromptRegistry(cfg *config.Config, logger *logrus.Logger) {
	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	if err := prompt.InitGlobal(context.Background(), promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("prompt registry initialization failed, fallback prompts will be used: %v", err)
	}
}

func initTenantPromptStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (prompt.TenantPromptStore, func() error, error) {
	tenantPromptStore, closer, err := newDBTenantPromptStore(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return tenantPromptStore, closer, nil
}

func attachTenantPromptStore(tenantPromptStore prompt.TenantPromptStore) error {
	if err := prompt.SetTenantPromptStore(tenantPromptStore); err != nil {
		return fmt.Errorf("attach tenant prompt store: %w", err)
	}
	return nil
}
