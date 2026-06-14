package listingkit

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
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

type taskManagedSubmissionWiring struct {
	assembly                         taskSubmissionAssembly
	orchestrator                     taskSubmissionOrchestratorWiring
	buildSheinSubmitOtherAPI         func(context.Context, *Task) (sheinother.OtherAPI, error)
	validateSheinPublishFreshness    func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	retrySheinSensitiveWordSubmit    func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	rememberSheinSubmittedResolution func(*Task, string)
}

type taskManagedSubmissionConfigWiring struct {
	support taskSubmissionSupportWiring
	managed taskManagedSubmissionWiring
}

type taskManagedSubmissionCollaboratorWiring struct {
	service  *service
	assembly taskSubmissionAssembly
	support  taskSubmissionSupportWiring
}

type taskManagedSubmissionCollaborators struct {
	recovery   *taskSubmissionRecoveryService
	direct     *taskDirectSubmissionService
	refresh    *taskSubmissionRefreshService
	submission *taskSubmissionService
}

type taskTemporalSubmissionWiring struct {
	assembly                         taskSubmissionAssembly
	orchestrator                     taskSubmissionOrchestratorWiring
	startSheinPublishWorkflow        func(context.Context, SheinPublishWorkflowStartInput) error
	loadSheinPublishTask             func(context.Context, string) (*Task, *SheinPackage, error)
	validateSheinPublishFreshness    func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	retrySheinSensitiveWordSubmit    func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	rememberSheinSubmittedResolution func(*Task, string)
}

type taskTemporalSubmissionConfigWiring struct {
	temporal    taskTemporalSubmissionWiring
	persistence *taskTemporalSubmissionPersistenceService
}

type taskTemporalSubmissionCollaborators struct {
	lifecycle   *taskTemporalSubmissionLifecycleService
	flow        *taskTemporalSubmissionFlowService
	persistence *taskTemporalSubmissionPersistenceService
	refresh     *taskTemporalSubmissionRefreshService
}

type taskTemporalSubmissionCollaboratorWiring struct {
	service *service
	wiring  taskTemporalSubmissionWiring
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

func (w taskSubmitTaskRecoveryCollaboratorWiring) resolve(existing submissionCollaborators) taskSubmitTaskRecoveryCollaborators {
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

func (w taskSubmissionCoreCollaboratorWiring) resolve(existing submissionCollaborators) taskSubmissionCoreCollaborators {
	execution := existing.taskSubmissionExecution
	if execution == nil {
		execution = w.newExecution()
	}
	state := existing.taskSubmissionState
	if state == nil {
		state = w.newState()
	}
	return taskSubmissionCoreCollaborators{
		execution: execution,
		state:     state,
	}
}

func buildTaskManagedSubmissionWiring(s *service) taskManagedSubmissionWiring {
	assembly := buildTaskSubmissionAssembly(s)
	return buildTaskManagedSubmissionWiringWithAssembly(s, assembly)
}

func buildTaskManagedSubmissionWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskManagedSubmissionWiring {
	return buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s, assembly, s.taskSubmissionRecoveryOrDefault())
}

func buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s *service, assembly taskSubmissionAssembly, recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {
	if assembly.resolver == nil {
		assembly = buildTaskSubmissionAssembly(s)
	}
	return taskManagedSubmissionWiring{
		assembly:                         assembly,
		orchestrator:                     buildTaskSubmissionOrchestratorWiringWithRecovery(s, assembly.resolver, recovery),
		buildSheinSubmitOtherAPI:         s.buildSheinSubmitOtherAPI,
		validateSheinPublishFreshness:    s.validateSheinPublishFreshness,
		retrySheinSensitiveWordSubmit:    s.retrySheinSensitiveWordSubmit,
		rememberSheinSubmittedResolution: s.rememberSheinSubmittedResolution,
	}
}

func buildTaskManagedSubmissionCollaboratorWiring(s *service) taskManagedSubmissionCollaboratorWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return taskManagedSubmissionCollaboratorWiring{
		service:  s,
		assembly: base.assembly,
		support:  base.support,
	}
}

func buildTaskManagedSubmissionConfigWiringWithRecovery(s *service, recovery *taskSubmissionRecoveryService) taskManagedSubmissionConfigWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return taskManagedSubmissionConfigWiring{
		support: base.support,
		managed: buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s, base.assembly, recovery),
	}
}

func (w taskManagedSubmissionCollaboratorWiring) newRecovery() *taskSubmissionRecoveryService {
	return newTaskSubmissionRecoveryService(buildTaskSubmissionRecoveryServiceConfigWithAssembly(w.service, w.assembly))
}

func (w taskManagedSubmissionCollaboratorWiring) buildManaged(recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {
	return buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(w.service, w.assembly, recovery)
}

func (w taskManagedSubmissionCollaboratorWiring) newDirect(managed taskManagedSubmissionWiring) *taskDirectSubmissionService {
	return newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfigWithWiring(
		managed,
	))
}

func (w taskManagedSubmissionCollaboratorWiring) newRefresh(managed taskManagedSubmissionWiring) *taskSubmissionRefreshService {
	return newTaskSubmissionRefreshService(buildTaskSubmissionRefreshServiceConfigWithWiring(
		managed,
	))
}

func (w taskManagedSubmissionCollaboratorWiring) newSubmission(recovery *taskSubmissionRecoveryService, direct *taskDirectSubmissionService) *taskSubmissionService {
	return newTaskSubmissionService(buildTaskSubmissionServiceConfigWithSupportAndCollaborators(
		w.support,
		w.service,
		recovery,
		direct,
	))
}

func (w taskManagedSubmissionCollaboratorWiring) resolve(existing submissionCollaborators) taskManagedSubmissionCollaborators {
	recovery := existing.taskSubmissionRecovery
	if recovery == nil {
		recovery = w.newRecovery()
	}
	managed := w.buildManaged(recovery)
	direct := existing.taskDirectSubmission
	if direct == nil {
		direct = w.newDirect(managed)
	}
	refresh := existing.taskSubmissionRefresh
	if refresh == nil {
		refresh = w.newRefresh(managed)
	}
	submission := existing.taskSubmission
	if submission == nil {
		submission = w.newSubmission(recovery, direct)
	}
	return taskManagedSubmissionCollaborators{
		recovery:   recovery,
		direct:     direct,
		refresh:    refresh,
		submission: submission,
	}
}

func buildTaskTemporalSubmissionWiring(s *service) taskTemporalSubmissionWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return buildTaskTemporalSubmissionWiringWithAssembly(s, base.assembly)
}

func buildTaskTemporalSubmissionConfigWiringWithPersistence(
	s *service,
	persistence *taskTemporalSubmissionPersistenceService,
) taskTemporalSubmissionConfigWiring {
	config := buildTaskTemporalSubmissionConfigWiring(s)
	config.persistence = persistence
	return config
}

func buildTaskTemporalSubmissionConfigWiring(s *service) taskTemporalSubmissionConfigWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return taskTemporalSubmissionConfigWiring{
		temporal: buildTaskTemporalSubmissionWiringWithAssembly(s, base.assembly),
	}
}

func buildTaskTemporalSubmissionCollaboratorWiring(s *service) taskTemporalSubmissionCollaboratorWiring {
	return taskTemporalSubmissionCollaboratorWiring{
		service: s,
		wiring:  buildTaskTemporalSubmissionWiring(s),
	}
}

func (w taskTemporalSubmissionCollaboratorWiring) newLifecycle() *taskTemporalSubmissionLifecycleService {
	return newTaskTemporalSubmissionLifecycleService(buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(w.wiring))
}

func (w taskTemporalSubmissionCollaboratorWiring) newFlow(persistence *taskTemporalSubmissionPersistenceService) *taskTemporalSubmissionFlowService {
	return newTaskTemporalSubmissionFlowService(buildTaskTemporalSubmissionFlowServiceConfigWithWiring(w.wiring, persistence))
}

func (w taskTemporalSubmissionCollaboratorWiring) newPersistence() *taskTemporalSubmissionPersistenceService {
	return newTaskTemporalSubmissionPersistenceService(buildTaskTemporalSubmissionPersistenceServiceConfigWithWiring(w.wiring))
}

func (w taskTemporalSubmissionCollaboratorWiring) newRefresh(persistence *taskTemporalSubmissionPersistenceService) *taskTemporalSubmissionRefreshService {
	return newTaskTemporalSubmissionRefreshService(buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(w.wiring, persistence))
}

func (w taskTemporalSubmissionCollaboratorWiring) resolve(existing submissionCollaborators) taskTemporalSubmissionCollaborators {
	persistence := existing.taskTemporalSubmissionPersistence
	if persistence == nil {
		persistence = w.newPersistence()
	}
	lifecycle := existing.taskTemporalSubmissionLifecycle
	if lifecycle == nil {
		lifecycle = w.newLifecycle()
	}
	flow := existing.taskTemporalSubmissionFlow
	if flow == nil {
		flow = w.newFlow(persistence)
	}
	refresh := existing.taskTemporalSubmissionRefresh
	if refresh == nil {
		refresh = w.newRefresh(persistence)
	}
	return taskTemporalSubmissionCollaborators{
		lifecycle:   lifecycle,
		flow:        flow,
		persistence: persistence,
		refresh:     refresh,
	}
}

func buildTaskTemporalSubmissionWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskTemporalSubmissionWiring {
	if assembly.resolver == nil {
		assembly = buildTaskSubmissionAssembly(s)
	}
	return taskTemporalSubmissionWiring{
		assembly:     assembly,
		orchestrator: buildTaskSubmissionOrchestratorWiring(s, assembly.resolver),
		startSheinPublishWorkflow: func(ctx context.Context, in SheinPublishWorkflowStartInput) error {
			client, _ := resolveSubmissionWorkflowClient(s)
			return client.StartSheinPublish(ctx, in)
		},
		loadSheinPublishTask:             s.loadSheinPublishTaskForTemporal,
		validateSheinPublishFreshness:    s.validateSheinPublishFreshness,
		retrySheinSensitiveWordSubmit:    s.retrySheinSensitiveWordSubmit,
		rememberSheinSubmittedResolution: s.rememberSheinSubmittedResolution,
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

func buildTaskSubmissionRecoveryServiceConfigWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionRecoveryServiceConfig {
	return taskSubmissionRecoveryServiceConfig{
		repo:                        assembly.repository.repo,
		buildTaskPreview:            assembly.preview.buildTaskPreview,
		buildSheinSubmitProductAPI:  assembly.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: assembly.bindings.state.persistSuccessfulSheinSubmission,
		recordSubmissionFailure:     assembly.bindings.state.recordSheinSubmissionFailureForState,
		resolveRemoteStatusCallback: resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfigWithSupportAndCollaborators(
	support taskSubmissionSupportWiring,
	s *service,
	recovery *taskSubmissionRecoveryService,
	direct *taskDirectSubmissionService,
) taskSubmissionServiceConfig {
	return taskSubmissionServiceConfig{
		repo:                            support.repo,
		lockSubmit:                      buildTaskSubmissionLockSubmit(s),
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		recovery:                        recovery,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           direct.submitSheinTaskDirect,
	}
}

func buildTaskSubmissionRefreshServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskSubmissionRefreshServiceConfig {
	return taskSubmissionRefreshServiceConfig{
		repo:                       wiring.assembly.repository.repo,
		lockSubmit:                 wiring.orchestrator.lockSubmit,
		buildTaskPreview:           wiring.assembly.preview.buildTaskPreview,
		buildSheinSubmitProductAPI: wiring.orchestrator.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   wiring.buildSheinSubmitOtherAPI,
		recovery:                   wiring.orchestrator.recovery,
		resolveRemoteStatus:        resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskDirectSubmissionServiceConfig {
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     wiring.assembly.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   wiring.validateSheinPublishFreshness,
		failSheinDirectSubmit:           wiring.assembly.bindings.state.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      wiring.assembly.bindings.execution.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   wiring.assembly.bindings.state.persistSheinDirectSubmitPhase,
		prepareSheinSubmitProduct:       wiring.assembly.bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:         wiring.assembly.bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:           wiring.assembly.bindings.resolver.resolveSubmitSettings,
		preValidateSheinSubmitProduct:   wiring.assembly.bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:        wiring.assembly.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:   wiring.retrySheinSensitiveWordSubmit,
		persistSuccessfulDirectResponse: wiring.assembly.bindings.state.persistSuccessfulSheinDirectResponse,
		finishSheinDirectSubmitAttempt:  wiring.assembly.bindings.state.finishSheinDirectSubmitAttempt,
		buildTaskPreview:                wiring.assembly.preview.buildTaskPreview,
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

func buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionLifecycleServiceConfig {
	return taskTemporalSubmissionLifecycleServiceConfig{
		startSheinPublishWorkflow:     wiring.startSheinPublishWorkflow,
		beginSheinSubmitLease:         wiring.orchestrator.recovery.beginSheinSubmitLease,
		loadSheinPublishTask:          wiring.loadSheinPublishTask,
		normalizeSheinSubmitPackage:   wiring.orchestrator.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness: wiring.validateSheinPublishFreshness,
		saveTaskResult:                wiring.assembly.repository.saveTaskResult,
		handleWorkflowStartFailure:    wiring.orchestrator.recovery.handleSheinWorkflowStartFailure,
		getTaskPreview:                wiring.assembly.preview.getTaskPreview,
	}
}

func buildTaskTemporalSubmissionFlowServiceConfigWithWiring(
	wiring taskTemporalSubmissionWiring,
	persistence *taskTemporalSubmissionPersistenceService,
) taskTemporalSubmissionFlowServiceConfig {
	return taskTemporalSubmissionFlowServiceConfig{
		loadSheinPublishTask:          wiring.loadSheinPublishTask,
		normalizeSheinSubmitPackage:   wiring.orchestrator.bindings.execution.normalizeSheinSubmitPackage,
		persistSheinSubmitPhase:       wiring.orchestrator.bindings.state.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:     wiring.orchestrator.bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:       wiring.orchestrator.bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:         wiring.orchestrator.bindings.resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:    wiring.orchestrator.bindings.execution.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct: wiring.orchestrator.bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:      wiring.orchestrator.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit: wiring.retrySheinSensitiveWordSubmit,
		persistence:                   persistence,
	}
}

func buildTaskTemporalSubmissionPersistenceServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionPersistenceServiceConfig {
	return taskTemporalSubmissionPersistenceServiceConfig{
		loadSheinPublishTask:                 wiring.loadSheinPublishTask,
		saveTaskResult:                       wiring.assembly.repository.saveTaskResult,
		persistSheinSubmitPhase:              wiring.orchestrator.bindings.state.persistSheinSubmitPhase,
		persistSuccessfulSheinSubmission:     wiring.orchestrator.bindings.state.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: wiring.orchestrator.bindings.state.recordSheinSubmissionFailureForState,
		rememberSheinSubmitted:               wiring.rememberSheinSubmittedResolution,
	}
}

func buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(
	wiring taskTemporalSubmissionWiring,
	persistence *taskTemporalSubmissionPersistenceService,
) taskTemporalSubmissionRefreshServiceConfig {
	return taskTemporalSubmissionRefreshServiceConfig{
		loadSheinPublishTask:           wiring.loadSheinPublishTask,
		buildSheinSubmitProductAPI:     wiring.orchestrator.bindings.execution.buildSheinSubmitProductAPI,
		persistSheinSubmitPhase:        wiring.orchestrator.bindings.state.persistSheinSubmitPhase,
		refreshSheinSubmitRemoteStatus: wiring.orchestrator.recovery.refreshSheinSubmitRemoteStatus,
		persistence:                    persistence,
	}
}
