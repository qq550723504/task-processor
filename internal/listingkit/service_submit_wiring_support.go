package listingkit

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

type taskSubmissionOrchestratorWiring struct {
	lockSubmit func(string) func()
	recovery   *taskSubmissionRecoveryService
	bindings   taskSubmissionBindings
}

type taskSubmissionRepositoryWiring struct {
	repo           Repository
	saveTaskResult func(context.Context, string, *ListingKitResult) error
}

type taskSubmitterWiring struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
}

func buildTaskSubmissionRepositoryWiring(s *service) taskSubmissionRepositoryWiring {
	if s == nil {
		return taskSubmissionRepositoryWiring{}
	}
	wiring := taskSubmissionRepositoryWiring{
		repo: s.repo,
	}
	if s.repo != nil {
		wiring.saveTaskResult = s.repo.SaveTaskResult
	}
	return wiring
}

func buildTaskSubmitterWiring(s *service) taskSubmitterWiring {
	repository := buildTaskSubmissionRepositoryWiring(s)
	return taskSubmitterWiring{
		repo: repository.repo,
		taskSubmitter: func() TaskSubmitter {
			return resolveTaskSubmitter(s)
		},
	}
}

func buildTaskSubmissionLockSubmit(s *service) func(string) func() {
	return func(key string) func() {
		return s.submission.sheinSubmitLocks.Lock(key)
	}
}

func buildTaskSubmissionOrchestratorWiring(s *service, resolver *submitRuntimeContextResolver) taskSubmissionOrchestratorWiring {
	return taskSubmissionOrchestratorWiring{
		lockSubmit: buildTaskSubmissionLockSubmit(s),
		recovery:   s.taskSubmissionRecoveryOrDefault(),
		bindings:   buildTaskSubmissionBindings(s, resolver),
	}
}

func resolveSubmissionStoreProfileRepo(s *service) StoreProfileRepository {
	if s == nil {
		return nil
	}
	if s.submissionDeps.storeProfileRepo != nil {
		s.storeProfileRepo = s.submissionDeps.storeProfileRepo
		return s.submissionDeps.storeProfileRepo
	}
	s.submissionDeps.storeProfileRepo = s.storeProfileRepo
	return s.storeProfileRepo
}

func resolveSubmissionStoreCatalog(s *service) SheinStoreCatalog {
	if s == nil {
		return nil
	}
	if s.submissionDeps.sheinStoreCatalog != nil {
		s.sheinStoreCatalog = s.submissionDeps.sheinStoreCatalog
		return s.submissionDeps.sheinStoreCatalog
	}
	s.submissionDeps.sheinStoreCatalog = s.sheinStoreCatalog
	return s.sheinStoreCatalog
}

func resolveSubmissionAPIClientFactory(s *service) SheinAPIClientFactory {
	if s == nil {
		return nil
	}
	if s.submissionDeps.sheinAPIClientFactory != nil {
		s.sheinAPIClientFactory = s.submissionDeps.sheinAPIClientFactory
		return s.submissionDeps.sheinAPIClientFactory
	}
	s.submissionDeps.sheinAPIClientFactory = s.sheinAPIClientFactory
	return s.sheinAPIClientFactory
}

func resolveSubmissionProductAPIBuilder(s *service) sheinpub.ProductAPIBuilder {
	if s == nil {
		return nil
	}
	if s.submissionDeps.sheinProductAPIBuilder != nil {
		s.sheinProductAPIBuilder = s.submissionDeps.sheinProductAPIBuilder
		return s.submissionDeps.sheinProductAPIBuilder
	}
	s.submissionDeps.sheinProductAPIBuilder = s.sheinProductAPIBuilder
	return s.sheinProductAPIBuilder
}

func resolveSubmissionImageAPIBuilder(s *service) sheinpub.ImageAPIBuilder {
	if s == nil {
		return nil
	}
	if s.submissionDeps.sheinImageAPIBuilder != nil {
		s.sheinImageAPIBuilder = s.submissionDeps.sheinImageAPIBuilder
		return s.submissionDeps.sheinImageAPIBuilder
	}
	s.submissionDeps.sheinImageAPIBuilder = s.sheinImageAPIBuilder
	return s.sheinImageAPIBuilder
}

func resolveSubmissionTranslateAPIBuilder(s *service) sheinpub.TranslateAPIBuilder {
	if s == nil {
		return nil
	}
	if s.submissionDeps.sheinTranslateAPIBuilder != nil {
		s.sheinTranslateAPIBuilder = s.submissionDeps.sheinTranslateAPIBuilder
		return s.submissionDeps.sheinTranslateAPIBuilder
	}
	s.submissionDeps.sheinTranslateAPIBuilder = s.sheinTranslateAPIBuilder
	return s.sheinTranslateAPIBuilder
}

func resolveSubmissionContentOptimizer(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	if s.submissionDeps.sheinContentOptimizer != nil {
		s.sheinContentOptimizer = s.submissionDeps.sheinContentOptimizer
		return s.submissionDeps.sheinContentOptimizer
	}
	s.submissionDeps.sheinContentOptimizer = s.sheinContentOptimizer
	return s.sheinContentOptimizer
}

func resolveSubmissionWorkflowClient(s *service) (SheinPublishWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	if s.submissionDeps.sheinPublishWorkflowClient != nil || s.submissionDeps.sheinPublishWorkflowEnabled {
		s.sheinPublishWorkflowClient = s.submissionDeps.sheinPublishWorkflowClient
		s.sheinPublishWorkflowEnabled = s.submissionDeps.sheinPublishWorkflowEnabled
		return s.submissionDeps.sheinPublishWorkflowClient, s.submissionDeps.sheinPublishWorkflowEnabled
	}
	s.submissionDeps.sheinPublishWorkflowClient = s.sheinPublishWorkflowClient
	s.submissionDeps.sheinPublishWorkflowEnabled = s.sheinPublishWorkflowEnabled
	return s.sheinPublishWorkflowClient, s.sheinPublishWorkflowEnabled
}
