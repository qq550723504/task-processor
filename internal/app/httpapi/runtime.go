package httpapi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	platformfeatureflag "task-processor/internal/platform/featureflag"
	platformobservability "task-processor/internal/platform/observability"
)

type runtimeDepsBuilders struct {
	buildTraceRuntime           func(context.Context, platformobservability.Config) (traceRuntime, error)
	buildFeatureFlagRuntime     func(context.Context, platformfeatureflag.Config) (featureFlagRuntime, error)
	migrateSchema               productListingSchemaMigrator
	buildProductCatalogDatabase productCatalogDatabaseBuilder
}

type featureFlagRuntime interface {
	BoolEvaluator
	Shutdown(context.Context) error
}

func buildRuntimeDeps(logger *logrus.Logger, configPath string) (*runtimeDeps, error) {
	return buildRuntimeDepsWithBuilders(logger, configPath, runtimeDepsBuilders{
		buildTraceRuntime:           buildPlatformTraceRuntime,
		buildFeatureFlagRuntime:     buildPlatformFeatureFlagRuntime,
		migrateSchema:               migrateProductListingAPIRuntimeSchema,
		buildProductCatalogDatabase: openProductCatalogDatabase,
	})
}

func buildRuntimeDepsWithSchemaMigrator(logger *logrus.Logger, configPath string, migrateSchema productListingSchemaMigrator) (*runtimeDeps, error) {
	return buildRuntimeDepsWithBuilders(logger, configPath, runtimeDepsBuilders{
		buildTraceRuntime:           buildPlatformTraceRuntime,
		buildFeatureFlagRuntime:     buildPlatformFeatureFlagRuntime,
		migrateSchema:               migrateSchema,
		buildProductCatalogDatabase: openProductCatalogDatabase,
	})
}

func buildRuntimeDepsWithBuilders(logger *logrus.Logger, configPath string, builders runtimeDepsBuilders) (*runtimeDeps, error) {
	timer := newStartupTimer(logger)

	done := timer.phase("loadConfig")
	cfg, err := loadHTTPAPIConfig(configPath)
	done()
	if err != nil {
		return nil, err
	}
	if builders.buildTraceRuntime == nil {
		return nil, fmt.Errorf("trace runtime builder is nil")
	}
	if builders.buildFeatureFlagRuntime == nil {
		return nil, fmt.Errorf("feature flag runtime builder is nil")
	}

	runtimeContext := context.Background()
	done = timer.phase("buildTraceRuntime")
	tracing, err := builders.buildTraceRuntime(runtimeContext, traceRuntimeConfig(cfg))
	done()
	if err != nil {
		return nil, fmt.Errorf("build trace runtime: %w", err)
	}
	if tracing == nil {
		return nil, fmt.Errorf("build trace runtime: runtime is nil")
	}
	traceCloser := func() error { return tracing.Shutdown(context.Background()) }
	ownedClosers := []func() error{traceCloser}
	completed := false
	defer func() {
		cleanupOwnedRuntimeResources(completed, ownedClosers)
	}()

	done = timer.phase("buildFeatureFlagRuntime")
	featureFlags, err := builders.buildFeatureFlagRuntime(runtimeContext, platformfeatureflag.Config{Flags: cfg.FeatureFlags.Flags})
	done()
	if err != nil {
		return nil, err
	}
	featureFlagsCloser := func() error { return featureFlags.Shutdown(context.Background()) }
	ownedClosers = append(ownedClosers, featureFlagsCloser)

	done = timer.phase("migrateProductListingSchema")
	err = migrateProductListingSchemaIfEnabled(runtimeContext, featureFlags, cfg.Database, logger, builders.migrateSchema)
	done()
	if err != nil {
		return nil, err
	}

	closers := make([]func() error, 0)
	var productCatalogDB *gorm.DB
	if builders.buildProductCatalogDatabase != nil {
		var databaseCloser func() error
		var databaseErr error
		productCatalogDB, databaseCloser, databaseErr = builders.buildProductCatalogDatabase(cfg.Database, logger)
		if databaseErr != nil {
			return nil, fmt.Errorf("build product catalog database: %w", databaseErr)
		}
		if databaseCloser != nil {
			ownedClosers = append(ownedClosers, databaseCloser)
			closers = append(closers, databaseCloser)
		}
	}

	done = timer.phase("buildPromptRuntimeDeps")
	promptDeps, err := buildPromptRuntimeDeps(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	ownedClosers = append(ownedClosers, promptDeps.closers...)
	done = timer.phase("buildOpenAIRuntimeDeps")
	openaiDeps, err := buildOpenAIRuntimeDeps(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	ownedClosers = append(ownedClosers, openaiDeps.closers...)
	closers = append(closers, openaiDeps.closers...)
	done = timer.phase("buildAICapabilityRuntimeDeps")
	aiCapabilityDeps, err := buildAICapabilityRuntimeDeps(cfg, logger)
	done()
	if err != nil {
		return nil, err
	}
	ownedClosers = append(ownedClosers, aiCapabilityDeps.closers...)
	closers = append(closers, aiCapabilityDeps.closers...)
	closers = append(closers, promptDeps.closers...)
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
			traceRuntime:         tracing,
			featureFlags:         featureFlags,
			closers:              closers,
			openaiMgr:            openaiDeps.openaiMgr,
			aiCredentialStore:    openaiDeps.aiCredentialStore,
			aiInvocationRecorder: aiCapabilityDeps.invocationRecorder,
			aiAsyncJobStore:      aiCapabilityDeps.asyncJobStore,
			tenantPromptStore:    promptDeps.tenantPromptStore,
			storeAPI:             storeAPI,
			productCatalogDB:     productCatalogDB,
		},
		features:            &featureRuntimeState{},
		constructionClosers: ownedClosers,
		featureFlagsCloser:  featureFlagsCloser,
		traceCloser:         traceCloser,
	}, nil
}

func buildPlatformTraceRuntime(ctx context.Context, cfg platformobservability.Config) (traceRuntime, error) {
	return platformobservability.NewTraceRuntime(ctx, cfg)
}

func buildPlatformFeatureFlagRuntime(ctx context.Context, cfg platformfeatureflag.Config) (featureFlagRuntime, error) {
	return platformfeatureflag.New(ctx, cfg)
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
