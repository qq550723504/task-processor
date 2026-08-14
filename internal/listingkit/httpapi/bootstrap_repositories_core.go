package httpapi

import (
	"fmt"

	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingsubscription"
)

func buildCoreRepositories(input BuildServiceInput, closers *closerStack) (*builtCoreRepositories, error) {
	taskRepos, err := buildCoreTaskRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	asyncRepos, err := buildCoreAsyncRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return &builtCoreRepositories{
		taskRepository:                taskRepos.taskRepository,
		studioAsyncJobRepository:      asyncRepos.studioAsyncJobRepository,
		studioBatchRepository:         asyncRepos.studioBatchRepository,
		studioBatchRunRepository:      asyncRepos.studioBatchRunRepository,
		studioBatchTaskLinkRepository: asyncRepos.studioBatchTaskLinkRepository,
		sheinSyncRepository:           asyncRepos.sheinSyncRepository,
	}, nil
}

func buildCoreTaskRepositories(input BuildServiceInput, closers *closerStack) (*coreTaskRepositories, error) {
	taskRepository, err := buildNamedWithClosers("core.task", input.Repositories.Core.Task, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	return &coreTaskRepositories{
		taskRepository: taskRepository,
	}, nil
}

func buildCoreAsyncRepositories(input BuildServiceInput, closers *closerStack) (*coreAsyncRepositories, error) {
	studioAsyncJobRepository, err := buildNamedWithClosers("core.studio_async_job", input.Repositories.Core.StudioAsyncJob, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioBatchRepository, err := buildNamedWithClosers("core.studio_batch", input.Repositories.Core.StudioBatch, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioBatchRunRepository, err := buildNamedWithClosers("core.studio_batch_run", input.Repositories.Core.StudioBatchRun, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioBatchTaskLinkRepository, err := buildNamedWithClosers("core.studio_batch_task_link", input.Repositories.Core.StudioBatchTaskLink, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	sheinSyncRepository, err := buildNamedWithClosers("core.shein_sync", input.Repositories.Core.SheinSync, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	return &coreAsyncRepositories{
		studioAsyncJobRepository:      studioAsyncJobRepository,
		studioBatchRepository:         studioBatchRepository,
		studioBatchRunRepository:      studioBatchRunRepository,
		studioBatchTaskLinkRepository: studioBatchTaskLinkRepository,
		sheinSyncRepository:           sheinSyncRepository,
	}, nil
}

func buildLateCoreRepositories(input BuildServiceInput, closers *closerStack) (*builtLateCoreRepositories, error) {
	subscriptionService, err := buildSubscriptionService(input, closers)
	if err != nil {
		return nil, err
	}
	memberInvitationAuditRepository, err := buildMemberInvitationAuditRepository(input, closers)
	if err != nil {
		return nil, err
	}
	dependencies, err := buildLateCoreRepositoryDependencies(input, closers)
	if err != nil {
		return nil, err
	}

	return &builtLateCoreRepositories{
		subscriptionService:             subscriptionService,
		memberInvitationAuditRepository: memberInvitationAuditRepository,
		assetRepository:                 dependencies.assetRepository,
		reviewRepository:                dependencies.reviewRepository,
		studioSessionRepository:         dependencies.studioSessionRepository,
		uploadedImageRepository:         dependencies.uploadedImageRepository,
		storeProfileRepository:          dependencies.storeProfileRepository,
		resolutionCacheStore:            dependencies.resolutionCacheStore,
	}, nil
}

func buildMemberInvitationAuditRepository(input BuildServiceInput, closers *closerStack) (memberinvite.AuditRepository, error) {
	builder := input.Repositories.Core.MemberInvitationAudit
	if builder == nil {
		return nil, nil
	}
	return buildNamedWithClosers("core.member_invitation_audit", builder, input.Config, input.Logger, closers)
}

func buildSubscriptionService(input BuildServiceInput, closers *closerStack) (*listingsubscription.Service, error) {
	subscriptionRepository, err := buildNamedWithClosers("core.subscription", input.Repositories.Core.Subscription, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	if input.Config != nil && input.Config.ListingKit.GenerationUsageLedgerEnabled {
		ledger, err := buildGenerationUsageLedger(subscriptionRepository)
		if err != nil {
			return nil, err
		}
		subscriptionService, err := listingsubscription.NewServiceWithLedger(subscriptionRepository, ledger)
		if err != nil {
			return nil, fmt.Errorf("create listing subscription service with usage ledger: %w", err)
		}
		return subscriptionService, nil
	}
	subscriptionService, err := listingsubscription.NewService(subscriptionRepository)
	if err != nil {
		return nil, fmt.Errorf("create listing subscription service: %w", err)
	}
	return subscriptionService, nil
}

func buildGenerationUsageLedger(repo listingsubscription.Repository) (listingsubscription.UsageLedger, error) {
	switch typed := repo.(type) {
	case *listingsubscription.GormRepository:
		return listingsubscription.NewGormUsageLedger(typed), nil
	case *listingsubscription.MemRepository:
		return listingsubscription.NewMemUsageLedger(typed), nil
	default:
		return nil, fmt.Errorf("generation usage ledger requires a supported subscription repository, got %T", repo)
	}
}

func buildLateCoreRepositoryDependencies(input BuildServiceInput, closers *closerStack) (*lateCoreRepositoryDependencies, error) {
	repoBuilders := input.Repositories.Core

	assetRepository, err := buildNamedWithClosers("core.asset", repoBuilders.Asset, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	reviewRepository, err := buildNamedWithClosers("core.review", repoBuilders.Review, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioSessionRepository, err := buildNamedWithClosers("core.studio_session", repoBuilders.StudioSession, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	uploadedImageRepository, err := buildNamedWithClosers("core.uploaded_image", repoBuilders.UploadedImage, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeProfileRepository, err := buildNamedWithClosers("core.store_profile", repoBuilders.StoreProfile, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	resolutionCacheStore, err := buildNamedWithClosers("core.shein_resolution_cache", repoBuilders.SheinResolutionCache, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &lateCoreRepositoryDependencies{
		assetRepository:         assetRepository,
		reviewRepository:        reviewRepository,
		studioSessionRepository: studioSessionRepository,
		uploadedImageRepository: uploadedImageRepository,
		storeProfileRepository:  storeProfileRepository,
		resolutionCacheStore:    resolutionCacheStore,
	}, nil
}
