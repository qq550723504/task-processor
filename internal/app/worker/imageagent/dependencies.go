package imageagentworker

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/configadapter"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent/assetpublication"
	"task-processor/internal/imageagent/objectstore"
	imageagentstore "task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	imageagenttools "task-processor/internal/imageagent/tools"
	"task-processor/internal/integration/httpimage"
	openaiclient "task-processor/internal/integration/openai"
	productassetpersistence "task-processor/internal/integration/persistence/product/asset"
	s3integration "task-processor/internal/integration/s3"
	platformdatabase "task-processor/internal/platform/database"
)

type imageCapabilityRuntime struct {
	OpenAIManager      *openaiclient.Manager
	CredentialResolver openaiclient.ClientConfigResolver
	InvocationRecorder aicapability.InvocationRecorder
	Logger             *logrus.Logger
}

type imageAgentWorkerDependencyResolver struct {
	LoadConfig         func(string) (*config.Config, error)
	OpenDB             func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB            func(*config.DatabaseConfig, *gorm.DB) error
	BuildAI            func(*config.Config, *gorm.DB, *logrus.Logger) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error)
	BuildCapabilities  func(imageCapabilityRuntime) (ImageCapabilities, error)
	BuildArtifactStore func(*config.Config, imageAgentArtifactTiming, *logrus.Logger) (imageagenttemporal.DurableArtifactStore, error)
	ArtifactTiming     imageAgentArtifactTiming
}

type imageAgentArtifactTiming struct {
	PublicationLeaseDuration time.Duration
	OperationTimeout         time.Duration
}

var defaultImageAgentArtifactTiming = imageAgentArtifactTiming{
	PublicationLeaseDuration: 2 * time.Minute,
	OperationTimeout:         time.Minute,
}

func defaultImageAgentWorkerDependencyResolver() imageAgentWorkerDependencyResolver {
	return imageAgentWorkerDependencyResolver{
		LoadConfig: config.LoadConfigFromFile,
		OpenDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenShared(configadapter.Database(cfg))
		},
		CloseDB: func(cfg *config.DatabaseConfig, db *gorm.DB) error {
			return platformdatabase.CloseShared(configadapter.Database(cfg), db)
		},
		BuildAI: buildImageAgentWorkerAI, BuildCapabilities: buildProductionImageCapabilities,
		BuildArtifactStore: buildImageAgentDurableArtifactStore,
		ArtifactTiming:     defaultImageAgentArtifactTiming,
	}
}

// ResolveImageAgentTemporalDependencies builds the dedicated production worker
// from the same database, ImageAgent artifact storage, and ListingKit result
// ownership adapters used by the API runtime.
func ResolveImageAgentTemporalDependencies(configPath string, logger *logrus.Logger) (appruntime.ImageAgentTemporalDependencies, func() error, error) {
	return ResolveImageAgentTemporalDependenciesForMode(configPath, logger, imageagenttemporal.WorkerWireModeV3)
}

// ResolveImageAgentTemporalDependenciesForMode composes only the capabilities
// registered by the selected worker process. The legacy public resolver above
// remains v3-compatible for existing callers.
func ResolveImageAgentTemporalDependenciesForMode(configPath string, logger *logrus.Logger, mode imageagenttemporal.WorkerWireMode) (appruntime.ImageAgentTemporalDependencies, func() error, error) {
	return resolveImageAgentTemporalDependenciesForMode(configPath, logger, mode, defaultImageAgentWorkerDependencyResolver())
}

func resolveImageAgentTemporalDependencies(configPath string, logger *logrus.Logger, resolver imageAgentWorkerDependencyResolver) (appruntime.ImageAgentTemporalDependencies, func() error, error) {
	return resolveImageAgentTemporalDependenciesForMode(configPath, logger, imageagenttemporal.WorkerWireModeV3, resolver)
}

func resolveImageAgentTemporalDependenciesForMode(configPath string, logger *logrus.Logger, mode imageagenttemporal.WorkerWireMode, resolver imageAgentWorkerDependencyResolver) (appruntime.ImageAgentTemporalDependencies, func() error, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent worker config path is required")
	}
	if _, err := mode.DefaultTaskQueue(); err != nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, err
	}
	cfg, err := resolver.LoadConfig(configPath)
	if err != nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("load image agent worker config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent worker database configuration is required")
	}
	var timing imageAgentArtifactTiming
	var artifactStore imageagenttemporal.DurableArtifactStore
	if mode == imageagenttemporal.WorkerWireModeV3 {
		timing = resolver.ArtifactTiming
		if timing == (imageAgentArtifactTiming{}) {
			timing = defaultImageAgentArtifactTiming
		}
		if err := timing.validate(); err != nil {
			return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("validate image agent durable artifact timing: %w", err)
		}
		if _, err := artifactStorageCapabilitiesFromConfig(cfg.ImageAgent.ArtifactStore); err != nil {
			return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("validate image agent durable artifact configuration: %w", err)
		}
		if resolver.BuildArtifactStore == nil {
			resolver.BuildArtifactStore = buildImageAgentDurableArtifactStore
		}
		artifactStore, err = resolver.BuildArtifactStore(cfg, timing, logger)
		if err != nil {
			return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent durable artifact store: %w", err)
		}
	}
	db, err := resolver.OpenDB(cfg.Database)
	if err != nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("open image agent worker database: %w", err)
	}
	closeDB := func() error { return resolver.CloseDB(cfg.Database, db) }
	manager, credentialResolver, recorder, err := resolver.BuildAI(cfg, db, logger)
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent provider runtime: %w", err)
	}
	if resolver.BuildCapabilities == nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent capability builder is required")
	}
	capabilities, err := resolver.BuildCapabilities(imageCapabilityRuntime{
		OpenAIManager: manager, CredentialResolver: credentialResolver, InvocationRecorder: recorder, Logger: logger,
	})
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent capabilities: %w", err)
	}
	if nilDependency(capabilities.SubjectExtractor) || nilDependency(capabilities.WhiteBackgroundRenderer) ||
		nilDependency(capabilities.SceneRenderer) || nilDependency(capabilities.Reviewer) ||
		nilDependency(capabilities.UsageQuoter) || nilDependency(capabilities.ProfileResolver) {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent capabilities are incomplete")
	}
	repository := imageagentstore.NewGormRepository(db)
	assetRepository, err := productassetpersistence.NewRepository(db)
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build product asset repository: %w", err)
	}
	publisher, err := assetpublication.NewV2Publisher(repository, assetRepository)
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent v2 asset publisher: %w", err)
	}
	executorDependencies := imageagenttools.Dependencies{
		SubjectExtractor: capabilities.SubjectExtractor, WhiteBackgroundRenderer: capabilities.WhiteBackgroundRenderer,
		SceneRenderer: capabilities.SceneRenderer, Reviewer: capabilities.Reviewer, UsageQuoter: capabilities.UsageQuoter,
		ProfileResolver: capabilities.ProfileResolver,
	}
	v2Executor := imageagenttools.NewFrozenV2ProductImageSlotExecutor(executorDependencies)
	v3Executor := imageagenttools.NewProductImageSlotExecutor(executorDependencies)
	dependencies := appruntime.ImageAgentTemporalDependencies{Repository: repository, SlotExecutor: v2Executor, Publisher: publisher}
	if mode == imageagenttemporal.WorkerWireModeV2 {
		return dependencies, closeDB, nil
	}
	publisherV3, err := assetpublication.NewPublisher(repository, assetRepository, artifactStore)
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent v3 asset publisher: %w", err)
	}
	dependencies.StagedSlotExecutor = v3Executor
	dependencies.ArtifactStore = artifactStore
	dependencies.PublisherV3 = publisherV3
	dependencies.PublicationLeaseDuration = timing.PublicationLeaseDuration
	return dependencies, closeDB, nil
}

func artifactStorageCapabilitiesFromConfig(store config.ImageAgentArtifactStoreConfig) (s3integration.ArtifactStorageCapabilities, error) {
	if !store.Enabled {
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact publication is disabled")
	}
	if strings.TrimSpace(store.Provider) != "s3" {
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact provider must be s3")
	}
	if strings.TrimSpace(store.S3.Bucket) == "" {
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact bucket is required")
	}
	if strings.TrimSpace(store.S3.Region) == "" {
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact region is required")
	}
	if strings.TrimSpace(store.PublicBase) == "" {
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact public base URL is required")
	}
	if strings.TrimSpace(store.S3.AccessKeyID) == "" || strings.TrimSpace(store.S3.SecretAccessKey) == "" {
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact access key ID and secret access key are both required")
	}
	switch strings.TrimSpace(store.S3.ArtifactMode) {
	case string(s3integration.ArtifactStorageModeAWS):
		return s3integration.ArtifactStorageCapabilities{Mode: s3integration.ArtifactStorageModeAWS}, nil
	case string(s3integration.ArtifactStorageModeCOS):
		if strings.TrimSpace(store.S3.Endpoint) == "" {
			return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact COS endpoint is required")
		}
		if !store.S3.COSImmutableNonVersionedBucketPolicy {
			return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact COS immutable non-versioned bucket policy must be confirmed")
		}
		return s3integration.ArtifactStorageCapabilities{Mode: s3integration.ArtifactStorageModeCOS, COSImmutableNonVersionedBucketPolicy: true}, nil
	default:
		return s3integration.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact mode must be explicitly aws or cos")
	}
}

func buildImageAgentDurableArtifactStore(cfg *config.Config, timing imageAgentArtifactTiming, logger *logrus.Logger) (imageagenttemporal.DurableArtifactStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("image agent configuration is required")
	}
	if err := timing.validate(); err != nil {
		return nil, err
	}
	storeConfig := cfg.ImageAgent.ArtifactStore
	capabilities, err := artifactStorageCapabilitiesFromConfig(storeConfig)
	if err != nil {
		return nil, err
	}
	s3Config := storeConfig.S3
	client, err := s3integration.NewClient(s3integration.ClientConfig{
		Region: s3Config.Region, Endpoint: s3Config.Endpoint, AccessKeyID: s3Config.AccessKeyID,
		SecretAccessKey: s3Config.SecretAccessKey, UsePathStyle: s3Config.UsePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("build durable image artifact S3 client: %w", err)
	}
	var componentLogger *logrus.Entry
	if logger != nil {
		componentLogger = logrus.NewEntry(logger).WithField("component", "image-agent-s3")
	}
	uploader, err := s3integration.NewUploaderWithOptions(client, s3integration.UploaderOptions{
		Bucket: s3Config.Bucket, PublicBase: storeConfig.PublicBase, Endpoint: s3Config.Endpoint,
		UsePathStyle: s3Config.UsePathStyle, ArtifactCapabilities: capabilities,
		Logger: s3integration.AdaptLogrus(componentLogger),
	})
	if err != nil {
		return nil, fmt.Errorf("build durable image artifact S3 uploader: %w", err)
	}
	store, err := objectstore.NewDurableArtifactStore(workerArtifactStore{uploader: uploader}, objectstore.DurableArtifactStoreConfig{MaxArtifactBytes: httpimage.DefaultMaxBodyBytes, OperationTimeout: timing.OperationTimeout})
	if err != nil {
		return nil, fmt.Errorf("build durable image artifact object store: %w", err)
	}
	return store, nil
}

func (timing imageAgentArtifactTiming) validate() error {
	if timing.PublicationLeaseDuration <= 0 || timing.OperationTimeout <= 0 || timing.OperationTimeout >= timing.PublicationLeaseDuration {
		return fmt.Errorf("durable image artifact operation timeout must be positive and strictly shorter than the publication lease")
	}
	return nil
}

func buildImageAgentWorkerAI(cfg *config.Config, db *gorm.DB, logger *logrus.Logger) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error) {
	if cfg == nil || db == nil {
		return nil, nil, nil, fmt.Errorf("image agent provider configuration and database are required")
	}
	var componentLogger *logrus.Entry
	if logger != nil {
		componentLogger = logrus.NewEntry(logger).WithField("component", "image-agent-openai")
	}
	manager, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients: cfg.OpenAI.ToClientConfigs(), DefaultClient: "default",
		Logger: openaiclient.AdaptLogrus(componentLogger),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	credentials := openaiclient.NewGormCredentialResolver(db)
	manager.SetConfigResolver(credentials)
	return manager, credentials, aicapabilitystore.NewGormInvocationRecorder(db), nil
}
