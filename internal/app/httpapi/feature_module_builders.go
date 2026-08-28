package httpapi

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	imageagenthttpapi "task-processor/internal/imageagent/httpapi"
	imageagentstore "task-processor/internal/imageagent/store"
	"task-processor/internal/infra/database"
	storageinfra "task-processor/internal/infra/storage"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	listingkitstore "task-processor/internal/listingkit/store"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	"task-processor/internal/sourceaccount"
)

type sourceAccountRepositoryBuilder func(*config.Config, *logrus.Logger) (sourceaccount.Repository, []func() error, error)

var buildSourceAccountRepository = listingkithttpapi.BuildSourceAccountRepository

type productModuleBuilder func(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error)

type imageModuleBuilder func(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error)

type amazonListingModuleBuilder func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)

type listingKitModuleBuilder func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)

type imageAgentModuleBuilder func(*config.Config, *logrus.Logger) (*imageagenthttpapi.BuildResult, error)

func buildProductModuleResult(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error) {
	return productenrichhttpapi.BuildRuntimeModule(input)
}

func buildImageModuleResult(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error) {
	return productimagehttpapi.BuildRuntimeModule(input)
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
	db, err := database.NewSharedDatabaseFromConfig(cfg.Database)
	if err != nil {
		closeWorkflowOnError()
		return nil, fmt.Errorf("build image agent repository: %w", err)
	}
	databaseCloser := func() error { return database.CloseSharedDatabase(cfg.Database, db) }
	service, err := imageagent.NewService(imageagentstore.NewGormRepository(db), workflowClient, listingkithttpapi.NewImageAgentAuthorizedAssetCatalog(listingkitstore.NewTaskRepository(db)))
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

func imageAgentDurableAssetPublicURLResolver(cfg *config.Config) imageagent.DurableAssetPublicURLResolver {
	if cfg == nil {
		return nil
	}
	publisher := cfg.ProductImage.Publisher
	if !publisher.Enabled || !strings.EqualFold(strings.TrimSpace(publisher.Provider), "s3") || strings.TrimSpace(publisher.PublicBase) == "" || strings.TrimSpace(publisher.S3.Bucket) == "" {
		return nil
	}
	return storageinfra.NewS3UploaderWithOptions(nil, storageinfra.S3UploaderOptions{
		Bucket: publisher.S3.Bucket, PublicBase: publisher.PublicBase,
		Endpoint: publisher.S3.Endpoint, UsePathStyle: publisher.S3.UsePathStyle,
	})
}
