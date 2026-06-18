package httpapi

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
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

	done = timer.phase("initPromptRegistry")
	initPromptRegistry(cfg, logger)
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
		tenantPromptStore, closer, err = initTenantPromptStore(cfg.Database, logger)
		done()
		if err != nil {
			return nil, fmt.Errorf("create tenant prompt store: %w", err)
		}
		done = timer.phase("attachTenantPromptStore")
		if err := attachTenantPromptStore(tenantPromptStore); err != nil {
			done()
			return nil, err
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
	done = timer.phase("buildProductEnrichRuntimeDeps")
	productEnrichDeps, err := buildProductEnrichRuntimeDeps(logger, cfg, openaiMgr)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildSharedResources")
	shared, err := buildHTTPAPISharedResources(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}

	timer.total("buildRuntimeDeps")
	return &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg:               cfg,
			closers:           closers,
			openaiMgr:         openaiMgr,
			aiCredentialStore: aiCredentialStore,
			tenantPromptStore: tenantPromptStore,
			llmMgr:            productEnrichDeps.llmMgr,
			inputParser:       productEnrichDeps.inputParser,
			understanding:     productEnrichDeps.understanding,
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
