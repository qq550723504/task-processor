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

type taskSubmissionServiceWiring struct {
	lockSubmit func(string) func()
	recovery   *taskSubmissionRecoveryService
	direct     *taskDirectSubmissionService
}

type taskSubmissionAssembly struct {
	preview    taskPreviewAccessWiring
	repository taskSubmissionRepositoryWiring
	resolver   *submitRuntimeContextResolver
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

func buildTaskSubmissionAssembly(s *service) taskSubmissionAssembly {
	resolver := buildSubmitRuntimeContextResolver(s)
	return buildTaskSubmissionAssemblyWithResolver(s, resolver)
}

func buildTaskSubmissionAssemblyWithResolver(s *service, resolver *submitRuntimeContextResolver) taskSubmissionAssembly {
	if resolver == nil {
		resolver = buildSubmitRuntimeContextResolver(s)
	}
	return taskSubmissionAssembly{
		preview:    buildTaskPreviewAccessWiring(s),
		repository: buildTaskSubmissionRepositoryWiring(s),
		resolver:   resolver,
		bindings:   buildTaskSubmissionBindings(s, resolver),
	}
}

func buildTaskSubmissionServiceWiring(s *service) taskSubmissionServiceWiring {
	return taskSubmissionServiceWiring{
		lockSubmit: buildTaskSubmissionLockSubmit(s),
		recovery:   s.taskSubmissionRecoveryOrDefault(),
		direct:     s.taskDirectSubmissionOrDefault(),
	}
}

func resolveSubmissionStoreProfileRepo(s *service) StoreProfileRepository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.storeProfileRepo, &s.mirrors.storeProfileRepo)
}

func resolveSubmissionStoreCatalog(s *service) SheinStoreCatalog {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.sheinStoreCatalog, &s.mirrors.sheinStoreCatalog)
}

func resolveSubmissionAPIClientFactory(s *service) SheinAPIClientFactory {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.sheinAPIClientFactory, &s.mirrors.sheinAPIClientFactory)
}

func resolveSubmissionProductAPIBuilder(s *service) sheinpub.ProductAPIBuilder {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.sheinProductAPIBuilder, &s.mirrors.sheinProductAPIBuilder)
}

func resolveSubmissionImageAPIBuilder(s *service) sheinpub.ImageAPIBuilder {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.sheinImageAPIBuilder, &s.mirrors.sheinImageAPIBuilder)
}

func resolveSubmissionTranslateAPIBuilder(s *service) sheinpub.TranslateAPIBuilder {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.sheinTranslateAPIBuilder, &s.mirrors.sheinTranslateAPIBuilder)
}

func resolveSubmissionContentOptimizer(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.submissionDeps.sheinContentOptimizer, &s.mirrors.sheinContentOptimizer)
}

func resolveSubmissionWorkflowClient(s *service) (SheinPublishWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return syncGroupedOptionalDependency(
		&s.submissionDeps.sheinPublishWorkflowClient,
		&s.submissionDeps.sheinPublishWorkflowEnabled,
		&s.runtime.sheinPublishWorkflowClient,
		&s.runtime.sheinPublishWorkflowEnabled,
	)
}
