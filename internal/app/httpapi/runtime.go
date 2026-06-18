package httpapi

import (
	"context"
	"fmt"
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
	timer := newStartupTimer(logger)

	done := timer.phase("loadConfig")
	cfg, err := config.LoadConfigFromFile(configPath)
	done()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	done = timer.phase("resolveImageWorkDir")
	imageWorkDir := resolveImageWorkDir(cfg)
	done()
	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	done = timer.phase("initPromptRegistry")
	if err := prompt.InitGlobal(context.Background(), promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("prompt registry initialization failed, fallback prompts will be used: %v", err)
	}
	done()

	done = timer.phase("createOpenAIManager")
	openaiMgr, err := newOpenAIManager(cfg.OpenAI)
	done()
	if err != nil {
		return nil, fmt.Errorf("create OpenAI manager: %w", err)
	}
	closers := make([]func() error, 0)
	var aiCredentialStore *openaiclient.GormCredentialResolver
	var tenantPromptStore prompt.TenantPromptStore
	if cfg.Database != nil {
		var closer func() error
		done = timer.phase("initTenantPromptStore")
		tenantPromptStore, closer, err = newDBTenantPromptStore(cfg.Database, logger)
		done()
		if err != nil {
			return nil, fmt.Errorf("create tenant prompt store: %w", err)
		}
		done = timer.phase("attachTenantPromptStore")
		if err := prompt.SetTenantPromptStore(tenantPromptStore); err != nil {
			done()
			return nil, fmt.Errorf("attach tenant prompt store: %w", err)
		}
		done()
		if closer != nil {
			closers = append(closers, closer)
		}

		done = timer.phase("initOpenAICredentialResolver")
		credentialResolver, closer, err := newDBOpenAICredentialResolver(cfg.Database, logger)
		done()
		if err != nil {
			return nil, fmt.Errorf("create OpenAI credential resolver: %w", err)
		}
		aiCredentialStore = credentialResolver
		done = timer.phase("attachOpenAICredentialResolver")
		openaiMgr.SetConfigResolver(credentialResolver)
		done()
		if closer != nil {
			closers = append(closers, closer)
		}
	}
	done = timer.phase("createLLMManager")
	llmMgr, err := productenrich.NewLLMManagerAdapterFromManager(openaiMgr)
	done()
	if err != nil {
		return nil, fmt.Errorf("create LLM manager: %w", err)
	}
	if cfg.Debug.ProductEnrichMockLLM {
		logger.WithField("config", "debug.productEnrichMockLLM").Warn("productenrich mock LLM enabled for local runtime")
		llmMgr = productenrich.NewLocalMockLLMManager()
	}
	if err := productenrich.ValidateMockLLMManager(llmMgr); err != nil {
		return nil, fmt.Errorf("validate LLM manager: %w", err)
	}

	done = timer.phase("createProductUnderstanding")
	productUnderstanding, err := productenrichenrich.NewProductUnderstanding(llmMgr)
	done()
	if err != nil {
		return nil, fmt.Errorf("create product understanding: %w", err)
	}

	done = timer.phase("createWebScraper")
	webScraper := newWebScraper(cfg)
	done()
	done = timer.phase("createInputParser")
	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	done()
	if err != nil {
		return nil, fmt.Errorf("create input parser: %w", err)
	}

	done = timer.phase("buildSharedResources")
	shared, err := appbootstrap.BuildSharedResources(cfg, logger, appbootstrap.SharedResourceOptions{
		AllowMissingManagementAuth: true,
		SkipManagementAuth:         true,
	})
	done()
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}

	timer.total("buildRuntimeDeps")
	return &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg:               cfg,
			closers:           closers,
			openaiMgr:         openaiMgr,
			aiCredentialStore: aiCredentialStore,
			tenantPromptStore: tenantPromptStore,
			llmMgr:            llmMgr,
			inputParser:       inputParser,
			understanding:     productUnderstanding,
			imageWorkDir:      imageWorkDir,
			sharedResources:   shared,
		},
		features: &featureRuntimeState{},
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
