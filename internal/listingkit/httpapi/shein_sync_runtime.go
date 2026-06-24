package httpapi

import (
	"task-processor/internal/listingkit"
)

type sheinSyncRuntimeServices struct {
	syncService              listingkit.SheinSyncService
	sdsRetirementSyncService listingkit.SheinSyncService
	candidateService         listingkit.SheinCandidateService
	enrollmentService        listingkit.SheinEnrollmentService
}

func buildSheinSyncRuntimeServices(input BuildServiceInput, repositories *builtRepositories, closers *closerStack) (sheinSyncRuntimeServices, error) {
	if repositories == nil || repositories.sheinSyncRepository == nil {
		return sheinSyncRuntimeServices{}, nil
	}

	productAPIBuilder := input.Hooks.SheinProductAPIBuilderFactory(repositories.storeRepository)
	syncService := listingkit.NewAsyncSheinSyncServiceWithBuilder(repositories.sheinSyncRepository, productAPIBuilder, nil)
	sdsRetirementSyncService := listingkit.NewSheinSyncServiceWithBuilder(repositories.sheinSyncRepository, productAPIBuilder, nil)
	candidateService := listingkit.NewSheinCandidateService(repositories.sheinSyncRepository)

	strategyProvider, err := buildSheinPromotionStrategyProvider(repositories)
	if err != nil {
		return sheinSyncRuntimeServices{}, err
	}
	enrollmentAdapter := buildSheinEnrollmentAdapter(input, repositories, strategyProvider)
	enrollmentService := listingkit.NewSheinEnrollmentService(repositories.sheinSyncRepository, enrollmentAdapter)

	return sheinSyncRuntimeServices{
		syncService:              syncService,
		sdsRetirementSyncService: sdsRetirementSyncService,
		candidateService:         candidateService,
		enrollmentService:        enrollmentService,
	}, nil
}
