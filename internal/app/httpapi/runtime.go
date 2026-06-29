package httpapi

import "github.com/sirupsen/logrus"

func buildRuntimeDeps(logger *logrus.Logger, configPath string) (*runtimeDeps, error) {
	timer := newStartupTimer(logger)

	done := timer.phase("loadConfig")
	cfg, err := loadHTTPAPIConfig(configPath)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("resolveImageWorkDir")
	imageWorkDir := resolveImageWorkDir(cfg)
	done()

	done = timer.phase("buildPromptRuntimeDeps")
	promptDeps, err := buildPromptRuntimeDeps(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	done = timer.phase("buildOpenAIRuntimeDeps")
	openaiDeps, err := buildOpenAIRuntimeDeps(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	closers := openaiDeps.closers
	closers = append(closers, promptDeps.closers...)
	done = timer.phase("buildProductEnrichRuntimeDeps")
	productEnrichDeps, err := buildProductEnrichRuntimeDeps(logger, cfg, openaiDeps.openaiMgr)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildSharedResources")
	storeAPI, err := buildHTTPAPISharedResources(cfg, logger)
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
			tenantPromptStore: promptDeps.tenantPromptStore,
			llmMgr:            productEnrichDeps.llmMgr,
			inputParser:       productEnrichDeps.inputParser,
			understanding:     productEnrichDeps.understanding,
			imageWorkDir:      imageWorkDir,
			storeAPI:          storeAPI,
		},
		features: &featureRuntimeState{},
	}, nil
}
