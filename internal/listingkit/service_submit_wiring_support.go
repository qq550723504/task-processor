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

type taskSubmissionSupportWiring struct {
	repo                     Repository
	resolveSheinStoreID      func(context.Context, *Task) (int64, error)
	resolveSubmitSettings    func(context.Context, *Task) SheinSettings
	sheinProductAPIBuilder   sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder     sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder sheinpub.TranslateAPIBuilder
	sheinContentOptimizer    openaiclient.ChatCompleter
	currentSheinPricingRule  func() sheinpub.PricingRule
	rememberSheinSubmitted   func(*Task, string)
}

type taskSubmissionBaseWiring struct {
	assembly taskSubmissionAssembly
	support  taskSubmissionSupportWiring
}

type taskSubmissionCoreCollaboratorWiring struct {
	service *service
	support taskSubmissionSupportWiring
}

type taskSubmissionCoreCollaborators struct {
	execution *taskSubmissionExecutionService
	state     *taskSubmissionStateService
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

type taskSubmitTaskRecoveryCollaboratorWiring struct {
	service   *service
	submitter taskSubmitterWiring
}

type taskSubmitTaskRecoveryCollaborators struct {
	taskRecovery *taskRecoveryService
	taskRequeue  *taskRequeueService
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

func buildTaskSubmitTaskRecoveryCollaboratorWiring(s *service) taskSubmitTaskRecoveryCollaboratorWiring {
	return taskSubmitTaskRecoveryCollaboratorWiring{
		service:   s,
		submitter: buildTaskSubmitterWiring(s),
	}
}

func (w taskSubmitTaskRecoveryCollaboratorWiring) newTaskRecovery() *taskRecoveryService {
	return newTaskRecoveryService(buildTaskRecoveryServiceConfigWithWiring(w.submitter))
}

func (w taskSubmitTaskRecoveryCollaboratorWiring) newTaskRequeue() *taskRequeueService {
	return newTaskRequeueService(buildTaskRequeueServiceConfigWithWiring(w.submitter))
}

func (w taskSubmitTaskRecoveryCollaboratorWiring) resolve(existing taskSubmitTaskRecoveryCollaborators) taskSubmitTaskRecoveryCollaborators {
	taskRecovery := existing.taskRecovery
	if taskRecovery == nil {
		taskRecovery = w.newTaskRecovery()
	}
	taskRequeue := existing.taskRequeue
	if taskRequeue == nil {
		taskRequeue = w.newTaskRequeue()
	}
	return taskSubmitTaskRecoveryCollaborators{
		taskRecovery: taskRecovery,
		taskRequeue:  taskRequeue,
	}
}

func buildTaskSubmissionLockSubmit(s *service) func(string) func() {
	return func(key string) func() {
		return s.submission.sheinSubmitLocks.Lock(key)
	}
}

func buildTaskSubmissionOrchestratorWiring(s *service, resolver *submitRuntimeContextResolver) taskSubmissionOrchestratorWiring {
	return buildTaskSubmissionOrchestratorWiringWithRecovery(s, resolver, s.taskSubmissionRecoveryOrDefault())
}

func buildTaskSubmissionOrchestratorWiringWithRecovery(
	s *service,
	resolver *submitRuntimeContextResolver,
	recovery *taskSubmissionRecoveryService,
) taskSubmissionOrchestratorWiring {
	return taskSubmissionOrchestratorWiring{
		lockSubmit: buildTaskSubmissionLockSubmit(s),
		recovery:   recovery,
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

func buildTaskSubmissionBaseWiring(s *service) taskSubmissionBaseWiring {
	assembly := buildTaskSubmissionAssembly(s)
	return buildTaskSubmissionBaseWiringWithAssembly(s, assembly)
}

func buildTaskSubmissionBaseWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionBaseWiring {
	if assembly.resolver == nil {
		assembly = buildTaskSubmissionAssembly(s)
	}
	return taskSubmissionBaseWiring{
		assembly: assembly,
		support:  buildTaskSubmissionSupportWiringWithAssembly(s, assembly),
	}
}

func buildTaskSubmissionSupportWiring(s *service) taskSubmissionSupportWiring {
	repository := buildTaskSubmissionRepositoryWiring(s)
	resolver := buildSubmitRuntimeContextResolver(s)
	wiring := taskSubmissionSupportWiring{
		repo:                     repository.repo,
		sheinProductAPIBuilder:   resolveSubmissionProductAPIBuilder(s),
		sheinImageAPIBuilder:     resolveSubmissionImageAPIBuilder(s),
		sheinTranslateAPIBuilder: resolveSubmissionTranslateAPIBuilder(s),
		sheinContentOptimizer:    resolveSubmissionContentOptimizer(s),
		currentSheinPricingRule:  s.currentSheinPricingRule,
		rememberSheinSubmitted:   s.rememberSheinSubmittedResolution,
	}
	if resolver != nil {
		wiring.resolveSheinStoreID = resolver.resolveStoreID
		wiring.resolveSubmitSettings = resolver.resolveSubmitSettings
	}
	return wiring
}

func buildTaskSubmissionSupportWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionSupportWiring {
	wiring := buildTaskSubmissionSupportWiring(s)
	if assembly.resolver != nil {
		wiring.resolveSheinStoreID = assembly.resolver.resolveStoreID
		wiring.resolveSubmitSettings = assembly.resolver.resolveSubmitSettings
	}
	if assembly.repository.repo != nil {
		wiring.repo = assembly.repository.repo
	}
	return wiring
}

func buildTaskSubmissionCoreCollaboratorWiring(s *service) taskSubmissionCoreCollaboratorWiring {
	return taskSubmissionCoreCollaboratorWiring{
		service: s,
		support: buildTaskSubmissionSupportWiring(s),
	}
}

func (w taskSubmissionCoreCollaboratorWiring) newExecution() *taskSubmissionExecutionService {
	return newTaskSubmissionExecutionService(buildTaskSubmissionExecutionServiceConfigWithSupport(w.support))
}

func (w taskSubmissionCoreCollaboratorWiring) newState() *taskSubmissionStateService {
	return newTaskSubmissionStateService(buildTaskSubmissionStateServiceConfigWithSupport(w.support))
}

func (w taskSubmissionCoreCollaboratorWiring) resolve(existing taskSubmissionCoreCollaborators) taskSubmissionCoreCollaborators {
	execution := existing.execution
	if execution == nil {
		execution = w.newExecution()
	}
	state := existing.state
	if state == nil {
		state = w.newState()
	}
	return taskSubmissionCoreCollaborators{
		execution: execution,
		state:     state,
	}
}

func resolveSubmissionStoreProfileRepo(s *service) StoreProfileRepository {
	if s == nil {
		return nil
	}
	return s.submissionDeps.storeProfileRepo
}

func resolveSubmissionStoreCatalog(s *service) SheinStoreCatalog {
	return resolveSheinStoreCatalog(s)
}

func resolveSubmissionAPIClientFactory(s *service) SheinAPIClientFactory {
	return resolveSheinAPIClientFactory(s)
}

func resolveSubmissionProductAPIBuilder(s *service) sheinpub.ProductAPIBuilder {
	if s == nil {
		return nil
	}
	return s.submissionDeps.sheinProductAPIBuilder
}

func resolveSubmissionImageAPIBuilder(s *service) sheinpub.ImageAPIBuilder {
	if s == nil {
		return nil
	}
	return s.submissionDeps.sheinImageAPIBuilder
}

func resolveSubmissionTranslateAPIBuilder(s *service) sheinpub.TranslateAPIBuilder {
	if s == nil {
		return nil
	}
	return s.submissionDeps.sheinTranslateAPIBuilder
}

func resolveSubmissionContentOptimizer(s *service) openaiclient.ChatCompleter {
	return resolveSheinContentOptimizer(s)
}

func resolveSubmissionWorkflowClient(s *service) (SheinPublishWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return s.submissionDeps.sheinPublishWorkflowClient, s.submissionDeps.sheinPublishWorkflowEnabled
}

func buildTaskRequeueServiceConfigWithWiring(wiring taskSubmitterWiring) taskRequeueServiceConfig {
	return taskRequeueServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
}

func buildTaskRecoveryServiceConfigWithWiring(wiring taskSubmitterWiring) taskRecoveryServiceConfig {
	return taskRecoveryServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
}

func buildTaskSubmissionExecutionServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionExecutionServiceConfig {
	return taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder:   wiring.sheinProductAPIBuilder,
		sheinImageAPIBuilder:     wiring.sheinImageAPIBuilder,
		sheinTranslateAPIBuilder: wiring.sheinTranslateAPIBuilder,
		sheinContentOptimizer:    wiring.sheinContentOptimizer,
		currentSheinPricingRule:  wiring.currentSheinPricingRule,
		resolveSheinStoreID:      wiring.resolveSheinStoreID,
		resolveSubmitSettings:    wiring.resolveSubmitSettings,
	}
}

func buildTaskSubmissionStateServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionStateServiceConfig {
	return taskSubmissionStateServiceConfig{
		repo:                   wiring.repo,
		rememberSheinSubmitted: wiring.rememberSheinSubmitted,
	}
}
