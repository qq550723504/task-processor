package imageagentworker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/core/config"
	imageagentstore "task-processor/internal/imageagent/store"
	imageagenttools "task-processor/internal/imageagent/tools"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/database"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	listingkitstore "task-processor/internal/listingkit/store"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type imageAgentWorkerDependencyResolver struct {
	LoadConfig        func(string) (*config.Config, error)
	OpenDB            func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB           func(*config.DatabaseConfig, *gorm.DB) error
	BuildAI           func(*config.Config, *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error)
	BuildCapabilities func(productimagehttpapi.RuntimeBuildInput) (productimagehttpapi.ImageAgentCapabilities, error)
}

func defaultImageAgentWorkerDependencyResolver() imageAgentWorkerDependencyResolver {
	return imageAgentWorkerDependencyResolver{
		LoadConfig: config.LoadConfigFromFile, OpenDB: database.NewSharedDatabaseFromConfig, CloseDB: database.CloseSharedDatabase,
		BuildAI: buildImageAgentWorkerAI, BuildCapabilities: productimagehttpapi.BuildImageAgentCapabilities,
	}
}

// ResolveImageAgentTemporalDependencies builds the dedicated production worker
// from the same database, ProductImage provider/storage, and ListingKit result
// ownership adapters used by the API runtime.
func ResolveImageAgentTemporalDependencies(configPath string, logger *logrus.Logger) (appruntime.ImageAgentTemporalDependencies, func() error, error) {
	return resolveImageAgentTemporalDependencies(configPath, logger, defaultImageAgentWorkerDependencyResolver())
}

func resolveImageAgentTemporalDependencies(configPath string, logger *logrus.Logger, resolver imageAgentWorkerDependencyResolver) (appruntime.ImageAgentTemporalDependencies, func() error, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent worker config path is required")
	}
	cfg, err := resolver.LoadConfig(configPath)
	if err != nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("load image agent worker config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return appruntime.ImageAgentTemporalDependencies{}, nil, fmt.Errorf("image agent worker database configuration is required")
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
	return appruntime.ImageAgentTemporalDependencies{Repository: repository, SlotExecutor: executor, Publisher: publisher}, closeDB, nil
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
