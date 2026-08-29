package imageagentworker

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent/objectstore"
	imageagentstore "task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	imageagenttools "task-processor/internal/imageagent/tools"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/storage"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/pkg/safeimagehttp"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type imageAgentWorkerDependencyResolver struct {
	LoadConfig         func(string) (*config.Config, error)
	OpenDB             func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB            func(*config.DatabaseConfig, *gorm.DB) error
	BuildAI            func(*config.Config, *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error)
	BuildCapabilities  func(productimagehttpapi.RuntimeBuildInput) (productimagehttpapi.ImageAgentCapabilities, error)
	BuildArtifactStore func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error)
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
		LoadConfig: config.LoadConfigFromFile, OpenDB: database.NewSharedDatabaseFromConfig, CloseDB: database.CloseSharedDatabase,
		BuildAI: buildImageAgentWorkerAI, BuildCapabilities: productimagehttpapi.BuildImageAgentCapabilities,
		BuildArtifactStore: buildImageAgentDurableArtifactStore,
		ArtifactTiming:     defaultImageAgentArtifactTiming,
	}
}

// ResolveImageAgentTemporalDependencies builds the dedicated production worker
// from the same database, ProductImage provider/storage, and ListingKit result
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
		if _, err := artifactStorageCapabilitiesFromConfig(cfg.ProductImage.Publisher); err != nil {
			return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("validate image agent durable artifact configuration: %w", err)
		}
		if resolver.BuildArtifactStore == nil {
			resolver.BuildArtifactStore = buildImageAgentDurableArtifactStore
		}
		artifactStore, err = resolver.BuildArtifactStore(cfg, timing)
		if err != nil {
			return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent durable artifact store: %w", err)
		}
	}
	db, err := resolver.OpenDB(cfg.Database)
	if err != nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("open image agent worker database: %w", err)
	}
	closeDB := func() error { return resolver.CloseDB(cfg.Database, db) }
	manager, credentialResolver, recorder, err := resolver.BuildAI(cfg, db)
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent provider runtime: %w", err)
	}
	workDir := strings.TrimSpace(cfg.ProductImage.WorkDir)
	if workDir == "" {
		workDir = filepath.Join(".", "tmp", "productimage")
	}
	capabilities, err := resolver.BuildCapabilities(productimagehttpapi.RuntimeBuildInput{
		Logger: logger, Config: cfg, OpenAIManager: manager, AICredentialResolver: credentialResolver,
		AIInvocationRecorder: recorder, ImageWorkDir: workDir,
	})
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("build image agent ProductImage capabilities: %w", err)
	}
	if capabilities.SubjectExtractor == nil || capabilities.WhiteBackgroundRenderer == nil || capabilities.SceneRenderer == nil || capabilities.AssetPublisher == nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent ProductImage capabilities are incomplete")
	}
	repository := imageagentstore.NewGormRepository(db)
	publisher, err := listingkithttpapi.NewImageAgentApprovedPublisher(repository, listingkitstore.NewImageAgentPublicationTransactionRepository(db))
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, err
	}
	executor := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor: capabilities.SubjectExtractor, WhiteBackgroundRenderer: capabilities.WhiteBackgroundRenderer,
		SceneRenderer: capabilities.SceneRenderer, AssetPublisher: capabilities.AssetPublisher,
	})
	dependencies := appruntime.ImageAgentTemporalDependencies{Repository: repository, SlotExecutor: executor, Publisher: publisher}
	if mode == imageagenttemporal.WorkerWireModeV2 {
		return dependencies, closeDB, nil
	}
	publisherV3, err := listingkithttpapi.NewImageAgentApprovedPublisherV3(repository, listingkitstore.NewImageAgentPublicationTransactionRepository(db), artifactStore)
	if err != nil {
		_ = closeDB()
		return appruntime.ImageAgentTemporalDependencies{}, nil, err
	}
	dependencies.StagedSlotExecutor = executor
	dependencies.ArtifactStore = artifactStore
	dependencies.PublisherV3 = publisherV3
	dependencies.PublicationLeaseDuration = timing.PublicationLeaseDuration
	return dependencies, closeDB, nil
}

func artifactStorageCapabilitiesFromConfig(publisher config.ProductImagePublisherConfig) (storage.ArtifactStorageCapabilities, error) {
	if !publisher.Enabled {
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact publication is disabled")
	}
	if strings.TrimSpace(publisher.Provider) != "s3" {
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact provider must be s3")
	}
	if strings.TrimSpace(publisher.S3.Bucket) == "" {
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact bucket is required")
	}
	if strings.TrimSpace(publisher.S3.Region) == "" {
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact region is required")
	}
	if strings.TrimSpace(publisher.PublicBase) == "" {
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact public base URL is required")
	}
	if strings.TrimSpace(publisher.S3.AccessKeyID) == "" || strings.TrimSpace(publisher.S3.SecretAccessKey) == "" {
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact access key ID and secret access key are both required")
	}
	switch strings.TrimSpace(publisher.S3.ArtifactMode) {
	case string(storage.ArtifactStorageModeAWS):
		return storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeAWS}, nil
	case string(storage.ArtifactStorageModeCOS):
		if strings.TrimSpace(publisher.S3.Endpoint) == "" {
			return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact COS endpoint is required")
		}
		if !publisher.S3.COSImmutableNonVersionedBucketPolicy {
			return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact COS immutable non-versioned bucket policy must be confirmed")
		}
		return storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeCOS, COSImmutableNonVersionedBucketPolicy: true}, nil
	default:
		return storage.ArtifactStorageCapabilities{}, fmt.Errorf("durable image artifact mode must be explicitly aws or cos")
	}
}

func buildImageAgentDurableArtifactStore(cfg *config.Config, timing imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("image agent configuration is required")
	}
	if err := timing.validate(); err != nil {
		return nil, err
	}
	publisher := cfg.ProductImage.Publisher
	capabilities, err := artifactStorageCapabilitiesFromConfig(publisher)
	if err != nil {
		return nil, err
	}
	s3Config := publisher.S3
	client, err := storage.NewS3Client(storage.S3ClientConfig{
		Region: s3Config.Region, Endpoint: s3Config.Endpoint, AccessKeyID: s3Config.AccessKeyID,
		SecretAccessKey: s3Config.SecretAccessKey, UsePathStyle: s3Config.UsePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("build durable image artifact S3 client: %w", err)
	}
	uploader := storage.NewS3UploaderWithOptions(client, storage.S3UploaderOptions{
		Bucket: s3Config.Bucket, PublicBase: publisher.PublicBase, Endpoint: s3Config.Endpoint,
		UsePathStyle: s3Config.UsePathStyle, ArtifactCapabilities: capabilities,
	})
	store, err := objectstore.NewS3DurableArtifactStore(uploader, objectstore.S3DurableArtifactStoreConfig{MaxArtifactBytes: safeimagehttp.DefaultMaxBodyBytes, OperationTimeout: timing.OperationTimeout})
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

func buildImageAgentWorkerAI(cfg *config.Config, db *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error) {
	if cfg == nil || db == nil {
		return nil, nil, nil, fmt.Errorf("image agent provider configuration and database are required")
	}
	manager, err := openaiclient.NewManager(&openaiclient.ManagerConfig{Clients: cfg.OpenAI.ToClientConfigs(), DefaultClient: "default"})
	if err != nil {
		return nil, nil, nil, err
	}
	credentials := openaiclient.NewGormCredentialResolver(db)
	manager.SetConfigResolver(credentials)
	return manager, credentials, aicapabilitystore.NewGormInvocationRecorder(db), nil
}
