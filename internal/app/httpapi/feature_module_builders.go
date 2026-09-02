package httpapi

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/app/configadapter"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	imageagenthttpapi "task-processor/internal/imageagent/httpapi"
	imageagentstore "task-processor/internal/imageagent/store"
	s3integration "task-processor/internal/integration/s3"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	listingkitstore "task-processor/internal/listingkit/store"
	platformdatabase "task-processor/internal/platform/database"
	"task-processor/internal/sourceaccount"
)

type sourceAccountRepositoryBuilder func(*config.Config, *logrus.Logger) (sourceaccount.Repository, []func() error, error)

var buildSourceAccountRepository = listingkithttpapi.BuildSourceAccountRepository

type amazonListingModuleBuilder func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)

type listingKitModuleBuilder func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)

type imageAgentModuleBuilder func(*config.Config, *logrus.Logger) (*imageagenthttpapi.BuildResult, error)

func attachImageAgentWorkspace(listingKitModule *listingkithttpapi.Module, imageAgentModule *imageagenthttpapi.BuildResult) error {
	if listingKitModule == nil || listingKitModule.TaskRepository == nil || imageAgentModule == nil || imageAgentModule.Application == nil {
		return nil
	}
	handler, err := listingkithttpapi.NewImageAgentWorkspaceHandler(listingKitModule.TaskRepository, imageAgentModule.Application)
	if err != nil {
		return err
	}
	listingKitModule.ImageAgentWorkspaceHandler = handler
	return nil
}

func buildAmazonListingModuleResult(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
	return amazonlistinghttpapi.BuildRuntimeModule(input)
}

func buildListingKitModuleResult(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
	return listingkithttpapi.BuildRuntimeModule(input)
}

func buildImageAgentModuleResult(cfg *config.Config, logger *logrus.Logger) (*imageagenthttpapi.BuildResult, error) {
	workflowClient, workflowCloser, err := appruntime.DialImageAgentTemporalWorkflowClient(logger)
	if err != nil {
		return nil, err
	}
	if workflowClient == nil {
		return nil, nil
	}
	closeWorkflowOnError := func() {
		if workflowCloser != nil {
			_ = workflowCloser()
		}
	}
	if cfg == nil || cfg.Database == nil {
		closeWorkflowOnError()
		return nil, fmt.Errorf("build image agent HTTP module: database config is required")
	}
	databaseConfig := configadapter.Database(cfg.Database)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		closeWorkflowOnError()
		return nil, fmt.Errorf("build image agent repository: %w", err)
	}
	databaseCloser := func() error { return platformdatabase.CloseShared(databaseConfig, db) }
	service, err := newImageAgentHTTPService(
		imageagentstore.NewGormRepository(db), workflowClient,
		listingkithttpapi.NewImageAgentAuthorizedAssetCatalog(listingkitstore.NewTaskRepository(db)),
	)
	if err != nil {
		_ = databaseCloser()
		closeWorkflowOnError()
		return nil, err
	}
	var handlerOptions []imageagenthttpapi.HandlerOption
	if publicURLs := imageAgentDurableAssetPublicURLResolver(cfg); publicURLs != nil {
		handlerOptions = append(handlerOptions, imageagenthttpapi.WithDurableAssetPublicURLResolver(publicURLs))
	}
	built, err := imageagenthttpapi.BuildModule(service, handlerOptions...)
	if err != nil {
		_ = databaseCloser()
		closeWorkflowOnError()
		return nil, err
	}
	built.Closers = append(built.Closers, databaseCloser)
	if workflowCloser != nil {
		built.Closers = append(built.Closers, workflowCloser)
	}
	return built, nil
}

func newImageAgentHTTPService(repository imageagent.Repository, workflows imageagent.WorkflowClient, catalog imageagent.AuthorizedAssetCatalog) (*imageagent.Service, error) {
	return imageagent.NewService(repository, workflows, catalog)
}

func imageAgentDurableAssetPublicURLResolver(cfg *config.Config) imageagent.DurableAssetPublicURLResolver {
	if cfg == nil {
		return nil
	}
	store := cfg.ImageAgent.ArtifactStore
	if !store.Enabled || !strings.EqualFold(strings.TrimSpace(store.Provider), "s3") || strings.TrimSpace(store.PublicBase) == "" || strings.TrimSpace(store.S3.Bucket) == "" {
		return nil
	}
	return imageAgentObjectURLResolver{
		bucket: store.S3.Bucket, publicBase: store.PublicBase,
		endpoint: store.S3.Endpoint, usePathStyle: store.S3.UsePathStyle,
	}
}

type imageAgentObjectURLResolver struct {
	bucket       string
	publicBase   string
	endpoint     string
	usePathStyle bool
}

func (r imageAgentObjectURLResolver) PublicURL(key string) string {
	fallbackBase := s3integration.BuildS3PublicBase(r.endpoint, r.bucket, r.usePathStyle)
	fallbackURL := ""
	if fallbackBase != "" {
		fallbackURL = strings.TrimRight(fallbackBase, "/") + "/" + strings.TrimLeft(strings.TrimSpace(key), "/")
	} else {
		fallbackURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", strings.TrimSpace(r.bucket), strings.TrimLeft(strings.TrimSpace(key), "/"))
	}
	return s3integration.ResolveObjectURL(r.publicBase, key, fallbackURL)
}
