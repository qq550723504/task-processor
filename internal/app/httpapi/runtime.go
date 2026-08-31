package httpapi

import (
	"context"

	"github.com/sirupsen/logrus"

	platformfeatureflag "task-processor/internal/platform/featureflag"
)

func buildRuntimeDeps(logger *logrus.Logger, configPath string) (*runtimeDeps, error) {
	timer := newStartupTimer(logger)

	done := timer.phase("loadConfig")
	cfg, err := loadHTTPAPIConfig(configPath)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildFeatureFlagRuntime")
	featureFlags, err := platformfeatureflag.New(context.Background(), platformfeatureflag.Config{Flags: cfg.FeatureFlags.Flags})
	done()
	if err != nil {
		return nil, err
	}
	featureFlagsCloser := func() error { return featureFlags.Shutdown(context.Background()) }
	ownedClosers := []func() error{featureFlagsCloser}
	completed := false
	defer func() {
		cleanupOwnedRuntimeResources(completed, ownedClosers)
	}()

	done = timer.phase("resolveImageWorkDir")
	imageWorkDir := resolveImageWorkDir(cfg)
	done()

	done = timer.phase("buildPromptRuntimeDeps")
	promptDeps, err := buildPromptRuntimeDeps(cfg, logger, featureFlags)
	done()
	if err != nil {
		return nil, err
	}
	ownedClosers = append(ownedClosers, promptDeps.closers...)
	done = timer.phase("buildOpenAIRuntimeDeps")
	openaiDeps, err := buildOpenAIRuntimeDeps(cfg, logger, featureFlags)
	done()
	if err != nil {
		return nil, err
	}
	ownedClosers = append(ownedClosers, openaiDeps.closers...)
	closers := make([]func() error, 0, len(openaiDeps.closers)+len(promptDeps.closers)+1)
	closers = append(closers, openaiDeps.closers...)
	done = timer.phase("buildAICapabilityRuntimeDeps")
	aiCapabilityDeps, err := buildAICapabilityRuntimeDeps(cfg, logger, featureFlags)
	done()
	if err != nil {
		return nil, err
	}
	ownedClosers = append(ownedClosers, aiCapabilityDeps.closers...)
	closers = append(closers, aiCapabilityDeps.closers...)
	closers = append(closers, promptDeps.closers...)
	done = timer.phase("buildProductEnrichRuntimeDeps")
	productEnrichDeps, err := buildProductEnrichRuntimeDeps(logger, cfg, openaiDeps.openaiMgr, openaiDeps.aiCredentialStore, aiCapabilityDeps.invocationRecorder)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildStoreAPI")
	storeAPI, storeCloser, err := buildHTTPAPIStoreAPI(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	if storeCloser != nil {
		ownedClosers = append(ownedClosers, storeCloser)
		closers = append(closers, storeCloser)
	}

	timer.total("buildRuntimeDeps")
	completed = true
	return &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg:                  cfg,
			featureFlags:         featureFlags,
			closers:              closers,
			openaiMgr:            openaiDeps.openaiMgr,
			aiCredentialStore:    openaiDeps.aiCredentialStore,
			aiInvocationRecorder: aiCapabilityDeps.invocationRecorder,
			aiAsyncJobStore:      aiCapabilityDeps.asyncJobStore,
			tenantPromptStore:    promptDeps.tenantPromptStore,
			llmMgr:               productEnrichDeps.llmMgr,
			inputParser:          productEnrichDeps.inputParser,
			understanding:        productEnrichDeps.understanding,
			contentGenerator:     productEnrichDeps.contentGenerator,
			specsGenerator:       productEnrichDeps.specsGenerator,
			variantsGenerator:    productEnrichDeps.variantsGenerator,
			fusionGenerator:      productEnrichDeps.fusionGenerator,
			scoringTextGenerator: productEnrichDeps.scoringTextGenerator,
			scoringImageAnalyzer: productEnrichDeps.scoringImageAnalyzer,
			imageWorkDir:         imageWorkDir,
			storeAPI:             storeAPI,
		},
		features:            &featureRuntimeState{},
		constructionClosers: ownedClosers,
		featureFlagsCloser:  featureFlagsCloser,
	}, nil
}

func cleanupOwnedRuntimeResources(completed bool, closers []func() error) {
	if completed {
		return
	}
	for index := len(closers) - 1; index >= 0; index-- {
		if closers[index] != nil {
			_ = closers[index]()
		}
	}
}
