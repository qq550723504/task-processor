package httpapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/prompt"
)

func buildRuntimeDeps(logger *logrus.Logger, configPath string) (*runtimeDeps, error) {
	cfg, err := config.LoadConfigFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	imageWorkDir := resolveImageWorkDir(cfg)
	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	if err := prompt.InitGlobal(context.Background(), promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("prompt registry initialization failed, fallback prompts will be used: %v", err)
	}

	openaiMgr, err := newOpenAIManager(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI manager: %w", err)
	}
	closers := make([]func() error, 0)
	var aiCredentialStore *openaiclient.GormCredentialResolver
	if cfg.Database != nil {
		tenantPromptStore, closer, err := newDBTenantPromptStore(cfg.Database, logger)
		if err != nil {
			return nil, fmt.Errorf("create tenant prompt store: %w", err)
		}
		if err := prompt.SetTenantPromptStore(tenantPromptStore); err != nil {
			return nil, fmt.Errorf("attach tenant prompt store: %w", err)
		}
		if closer != nil {
			closers = append(closers, closer)
		}

		credentialResolver, closer, err := newDBOpenAICredentialResolver(cfg.Database, logger)
		if err != nil {
			return nil, fmt.Errorf("create OpenAI credential resolver: %w", err)
		}
		aiCredentialStore = credentialResolver
		openaiMgr.SetConfigResolver(credentialResolver)
		if closer != nil {
			closers = append(closers, closer)
		}
	}
	llmMgr, err := productenrich.NewLLMManagerAdapterFromManager(openaiMgr)
	if err != nil {
		return nil, fmt.Errorf("create LLM manager: %w", err)
	}
	if productenrich.IsMockLLMEnabled(os.Getenv(productenrich.ProductEnrichMockLLMEnv)) {
		logger.WithField("env", productenrich.ProductEnrichMockLLMEnv).Warn("productenrich mock LLM enabled for local runtime")
		llmMgr = productenrich.NewLocalMockLLMManager()
	}
	if err := productenrich.ValidateMockLLMManager(llmMgr); err != nil {
		return nil, fmt.Errorf("validate LLM manager: %w", err)
	}

	productUnderstanding, err := productenrichenrich.NewProductUnderstanding(llmMgr)
	if err != nil {
		return nil, fmt.Errorf("create product understanding: %w", err)
	}

	webScraper := newWebScraper(cfg)
	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	if err != nil {
		return nil, fmt.Errorf("create input parser: %w", err)
	}

	shared, err := appbootstrap.BuildSharedResources(cfg, logger, appbootstrap.SharedResourceOptions{
		AllowMissingManagementAuth: true,
		SkipManagementAuth:         true,
	})
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}

	return &runtimeDeps{
		cfg:               cfg,
		closers:           closers,
		openaiMgr:         openaiMgr,
		aiCredentialStore: aiCredentialStore,
		llmMgr:            llmMgr,
		inputParser:       inputParser,
		understanding:     productUnderstanding,
		imageWorkDir:      imageWorkDir,
		shared:            shared,
		managementClient:  shared.ManagementClient,
	}, nil
}

func resolveImageWorkDir(cfg *config.Config) string {
	if cfg == nil {
		return filepath.Join(".", "tmp", "productimage")
	}

	workDir := filepath.Clean(cfg.ProductImage.WorkDir)
	if workDir == "" || workDir == "." {
		return filepath.Join(".", "tmp", "productimage")
	}

	return workDir
}
