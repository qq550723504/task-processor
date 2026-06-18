package httpapi

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
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

	done = timer.phase("buildOpenAIRuntimeDeps")
	openaiDeps, err := buildOpenAIRuntimeDeps(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	closers := openaiDeps.closers
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
	}
	done = timer.phase("buildProductEnrichRuntimeDeps")
	productEnrichDeps, err := buildProductEnrichRuntimeDeps(logger, cfg, openaiDeps.openaiMgr)
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
			openaiMgr:         openaiDeps.openaiMgr,
			aiCredentialStore: openaiDeps.aiCredentialStore,
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
