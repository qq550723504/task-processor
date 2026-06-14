package listingkit

import (
	"context"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	studiodomain "task-processor/internal/listing/studio"
)

type taskStudioSessionRepoWiring struct {
	repo StudioSessionRepository
}

type taskStudioSessionWiring struct {
	repo StudioSessionRepository
}

type taskStudioSessionConfigWiring struct {
	session                  taskStudioSessionWiring
	runner                   *listingStudioSessionRunner
	asyncJobRunner           *listingStudioSessionAsyncJobRunner
	generationMetadataRunner *listingStudioSessionGenerationMetadataRunner
	reviewTaskMetadataRunner *listingStudioSessionReviewTaskMetadataRunner
	generalMetadataRunner    *listingStudioSessionGeneralMetadataRunner
	batchDraftRunner         *listingStudioBatchDraftRunner
}

type taskStudioSessionCollaboratorWiring struct {
	service *service
}

type taskStudioSessionCollaborators struct {
	session    *taskStudioSessionService
	batchDraft *taskStudioBatchDraftService
	media      *taskStudioMediaService
}

type taskStudioBatchServiceWiring struct {
	repo               StudioBatchRepository
	studioSessionRepo  StudioSessionRepository
	generator          *studioBatchGenerationService
	createGenerateTask func(context.Context, *GenerateRequest) (*Task, error)
	getTask            func(context.Context, string) (*Task, error)
	ensureGraph        func(context.Context, string) error
	loadDetail         func(context.Context, string) (*StudioBatchDetail, error)
	resetRetryItems    func(context.Context, []StudioBatchItemRecord) error
	currentTime        func() time.Time
}

type taskStudioBatchServiceConfigWiring struct {
	batch        taskStudioBatchServiceWiring
	detailRunner *listingStudioBatchDetailRunner
	reviewRunner *listingStudioBatchReviewRunner
	retryRunner  *listingStudioBatchRetryPrepareRunner
	taskPrepare  *listingStudioBatchTaskPrepareRunner
	taskResume   *listingStudioBatchTaskResumeRunner
}

type taskStudioBatchCollaboratorWiring struct {
	service *service
}

type taskStudioBatchCollaborators struct {
	batchGeneration *studioBatchGenerationService
	batch           *taskStudioBatchService
}

type taskStudioMediaWiring struct {
	imageGenerator        openaiclient.ImageGenerator
	promptDiversifier     openaiclient.ChatCompleter
	uploadStoreConfigured bool
	uploadImages          func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
}

type studioBatchGenerationWiring struct {
	repo    StudioBatchRepository
	execute func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error)
}

type taskStudioBatchRunRepoWiring struct {
	repo              StudioBatchRunRepository
	studioSessionRepo StudioSessionRepository
}

type taskStudioBatchRunWiring struct {
	repo              StudioBatchRunRepository
	studioSessionRepo StudioSessionRepository
}

type taskStudioBatchRunConfigWiring struct {
	batchRun taskStudioBatchRunWiring
}

type taskStudioBatchRunCollaboratorWiring struct {
	service *service
	wiring  taskStudioBatchRunWiring
}

type taskStudioBatchRunCollaborators struct {
	batchRun       *taskStudioBatchRunService
	runExecutor    *taskStudioBatchRunExecutor
	runCoordinator *studioBatchRunCoordinator
}

func buildTaskStudioSessionRepoWiring(s *service) taskStudioSessionRepoWiring {
	return taskStudioSessionRepoWiring{
		repo: resolveStudioSessionRepo(s),
	}
}

func buildTaskStudioSessionWiring(s *service) taskStudioSessionWiring {
	repoWiring := buildTaskStudioSessionRepoWiring(s)
	return taskStudioSessionWiring{
		repo: repoWiring.repo,
	}
}

func (w taskStudioSessionWiring) newSessionRunner() *listingStudioSessionRunner {
	return newListingStudioSessionService(w.repo)
}

func (w taskStudioSessionWiring) newAsyncJobRunner() *listingStudioSessionAsyncJobRunner {
	return newListingStudioSessionAsyncJobService(w.repo)
}

func (w taskStudioSessionWiring) newGenerationMetadataRunner() *listingStudioSessionGenerationMetadataRunner {
	return newListingStudioSessionGenerationMetadataService(w.repo)
}

func (w taskStudioSessionWiring) newReviewTaskMetadataRunner() *listingStudioSessionReviewTaskMetadataRunner {
	return newListingStudioSessionReviewTaskMetadataService(w.repo)
}

func (w taskStudioSessionWiring) newGeneralMetadataRunner() *listingStudioSessionGeneralMetadataRunner {
	return newListingStudioSessionGeneralMetadataService(w.repo)
}

func (w taskStudioSessionWiring) newBatchDraftRunner() *listingStudioBatchDraftRunner {
	return newListingStudioBatchDraftService(w.repo)
}

func buildTaskStudioSessionConfigWiring(s *service) taskStudioSessionConfigWiring {
	session := buildTaskStudioSessionWiring(s)
	return taskStudioSessionConfigWiring{
		session:                  session,
		runner:                   session.newSessionRunner(),
		asyncJobRunner:           session.newAsyncJobRunner(),
		generationMetadataRunner: session.newGenerationMetadataRunner(),
		reviewTaskMetadataRunner: session.newReviewTaskMetadataRunner(),
		generalMetadataRunner:    session.newGeneralMetadataRunner(),
		batchDraftRunner:         session.newBatchDraftRunner(),
	}
}

func buildTaskStudioSessionCollaboratorWiring(s *service) taskStudioSessionCollaboratorWiring {
	return taskStudioSessionCollaboratorWiring{service: s}
}

func (w taskStudioSessionCollaboratorWiring) newSession() *taskStudioSessionService {
	return newTaskStudioSessionService(buildTaskStudioSessionServiceConfig(w.service))
}

func (w taskStudioSessionCollaboratorWiring) newBatchDraft() *taskStudioBatchDraftService {
	return newTaskStudioBatchDraftService(buildTaskStudioBatchDraftServiceConfig(w.service))
}

func (w taskStudioSessionCollaboratorWiring) newMedia() *taskStudioMediaService {
	return newTaskStudioMediaService(buildTaskStudioMediaServiceConfig(w.service))
}

func (w taskStudioSessionCollaboratorWiring) resolve(existing studioCollaborators) taskStudioSessionCollaborators {
	session := existing.session
	if session == nil {
		session = w.newSession()
	}
	batchDraft := existing.batchDraft
	if batchDraft == nil {
		batchDraft = w.newBatchDraft()
	}
	media := existing.media
	if media == nil {
		media = w.newMedia()
	}
	return taskStudioSessionCollaborators{
		session:    session,
		batchDraft: batchDraft,
		media:      media,
	}
}

func buildTaskStudioBatchRunRepoWiring(s *service) taskStudioBatchRunRepoWiring {
	return taskStudioBatchRunRepoWiring{
		repo:              resolveStudioBatchRunRepo(s),
		studioSessionRepo: resolveStudioSessionRepo(s),
	}
}

func buildTaskStudioBatchRunWiring(s *service) taskStudioBatchRunWiring {
	repoWiring := buildTaskStudioBatchRunRepoWiring(s)
	return taskStudioBatchRunWiring{
		repo:              repoWiring.repo,
		studioSessionRepo: repoWiring.studioSessionRepo,
	}
}

func (w taskStudioBatchRunWiring) newServiceRunner(startRun func(context.Context, string) error) *studiodomain.BatchRunService {
	return newListingStudioBatchRunService(w.repo, w.studioSessionRepo, startRun)
}

func (w taskStudioBatchRunWiring) newCompletionRunner(now func() time.Time) *listingStudioBatchRunCompletionRunner {
	return newListingStudioBatchRunCompletionService(w.repo, now)
}

func buildTaskStudioBatchRunConfigWiring(s *service) taskStudioBatchRunConfigWiring {
	return taskStudioBatchRunConfigWiring{
		batchRun: buildTaskStudioBatchRunWiring(s),
	}
}

func buildTaskStudioBatchRunCollaboratorWiring(s *service) taskStudioBatchRunCollaboratorWiring {
	return taskStudioBatchRunCollaboratorWiring{
		service: s,
		wiring:  buildTaskStudioBatchRunWiring(s),
	}
}

func (w taskStudioBatchRunCollaboratorWiring) newRunExecutor() *taskStudioBatchRunExecutor {
	if resolveStudioBatchRunRepo(w.service) == nil || resolveStudioSessionRepo(w.service) == nil {
		return nil
	}
	if resolveStudioImageGenerator(w.service) == nil || resolveStudioUploadStore(w.service) == nil {
		return nil
	}
	return newTaskStudioBatchRunExecutor(buildTaskStudioBatchRunExecutorConfigWithWiring(w.service, w.wiring))
}

func (w taskStudioBatchRunCollaboratorWiring) newRunCoordinator(executor *taskStudioBatchRunExecutor) *studioBatchRunCoordinator {
	if executor == nil {
		return nil
	}
	return newStudioBatchRunCoordinator(buildStudioBatchRunCoordinatorConfigWithCollaborators(w.wiring, executor))
}

func (w taskStudioBatchRunCollaboratorWiring) newBatchRun(coordinator *studioBatchRunCoordinator) *taskStudioBatchRunService {
	return newTaskStudioBatchRunService(buildTaskStudioBatchRunServiceConfigWithCollaborators(w.wiring, coordinator))
}

func (w taskStudioBatchRunCollaboratorWiring) resolve(existing studioCollaborators) taskStudioBatchRunCollaborators {
	runExecutor := existing.runExecutor
	if runExecutor == nil {
		runExecutor = w.newRunExecutor()
	}
	runCoordinator := existing.runCoordinator
	if runCoordinator == nil {
		runCoordinator = w.newRunCoordinator(runExecutor)
	}
	batchRun := existing.batchRun
	if batchRun == nil {
		batchRun = w.newBatchRun(runCoordinator)
	}
	return taskStudioBatchRunCollaborators{
		batchRun:       batchRun,
		runExecutor:    runExecutor,
		runCoordinator: runCoordinator,
	}
}

func buildTaskStudioBatchServiceWiringWithGenerator(s *service, generator *studioBatchGenerationService) taskStudioBatchServiceWiring {
	if s == nil {
		return taskStudioBatchServiceWiring{}
	}
	repository := buildServiceRepositoryWiring(s)
	repo := resolveStudioBatchRepo(s)
	studioSessionRepo := resolveStudioSessionRepo(s)
	return taskStudioBatchServiceWiring{
		repo:               repo,
		studioSessionRepo:  studioSessionRepo,
		generator:          generator,
		createGenerateTask: s.CreateGenerateTask,
		getTask:            repository.getTask,
		ensureGraph: func(ctx context.Context, batchID string) error {
			return ensureStudioBatchGenerationGraphForResume(ctx, repo, studioSessionRepo, time.Now, batchID)
		},
		loadDetail: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
		},
		resetRetryItems: func(ctx context.Context, items []StudioBatchItemRecord) error {
			batchService := &taskStudioBatchService{
				repo:        repo,
				currentTime: time.Now,
			}
			return batchService.resetStudioBatchRetryItems(ctx, items)
		},
		currentTime: time.Now,
	}
}

func buildTaskStudioBatchServiceWiring(s *service) taskStudioBatchServiceWiring {
	return buildTaskStudioBatchServiceWiringWithGenerator(s, s.studioBatchGenerationOrDefault())
}

func (w taskStudioBatchServiceWiring) newDetailRunner() *listingStudioBatchDetailRunner {
	return newListingStudioBatchDetailService(w.repo, w.studioSessionRepo, w.ensureGraph)
}

func (w taskStudioBatchServiceWiring) newReviewRunner() *listingStudioBatchReviewRunner {
	return newListingStudioBatchReviewService(w.repo, w.loadDetail, w.currentTime)
}

func buildTaskStudioBatchServiceConfigWiringWithGenerator(s *service, generator *studioBatchGenerationService) taskStudioBatchServiceConfigWiring {
	batch := buildTaskStudioBatchServiceWiringWithGenerator(s, generator)
	var updateSession func(context.Context, *SheinStudioSession) error
	if sessionUpdater, ok := batch.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		updateSession = sessionUpdater.UpdateSession
	}
	var updateBatch func(context.Context, *StudioBatchRecord) error
	if batch.repo != nil {
		updateBatch = batch.repo.UpdateStudioBatch
	}
	return taskStudioBatchServiceConfigWiring{
		batch:        batch,
		detailRunner: batch.newDetailRunner(),
		reviewRunner: batch.newReviewRunner(),
		retryRunner:  newListingStudioBatchRetryPrepareService(batch.repo, batch.loadDetail, batch.resetRetryItems),
		taskPrepare: newListingStudioBatchTaskPrepareService(
			updateSession,
			updateBatch,
			func(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
				detail, err := batch.loadDetail(ctx, batchID)
				if err != nil {
					return nil, err
				}
				return &CreateStudioBatchTasksResult{
					Batch:        detail.Batch,
					Items:        detail.Items,
					CreatedTasks: detail.CreatedTasks,
					FailedTasks:  detail.FailedTasks,
				}, nil
			},
			batch.currentTime,
		),
		taskResume: newListingStudioBatchTaskResumeService(
			updateSession,
			updateBatch,
			func(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
				detail, err := batch.loadDetail(ctx, batchID)
				if err != nil {
					return nil, err
				}
				return &CreateStudioBatchTasksResult{
					Batch:        detail.Batch,
					Items:        detail.Items,
					CreatedTasks: detail.CreatedTasks,
					FailedTasks:  detail.FailedTasks,
				}, nil
			},
			batch.currentTime,
		),
	}
}

func buildTaskStudioBatchServiceConfigWiring(s *service) taskStudioBatchServiceConfigWiring {
	return buildTaskStudioBatchServiceConfigWiringWithGenerator(s, s.studioBatchGenerationOrDefault())
}

func buildTaskStudioBatchCollaboratorWiring(s *service) taskStudioBatchCollaboratorWiring {
	return taskStudioBatchCollaboratorWiring{service: s}
}

func (w taskStudioBatchCollaboratorWiring) newBatchGeneration() *studioBatchGenerationService {
	return newStudioBatchGenerationService(buildStudioBatchGenerationServiceConfig(w.service))
}

func (w taskStudioBatchCollaboratorWiring) newBatch(batchGeneration *studioBatchGenerationService) *taskStudioBatchService {
	return newTaskStudioBatchService(buildTaskStudioBatchServiceConfigWithCollaborators(
		buildTaskStudioBatchServiceConfigWiringWithGenerator(w.service, batchGeneration),
	))
}

func (w taskStudioBatchCollaboratorWiring) resolve(existing studioCollaborators) taskStudioBatchCollaborators {
	batchGeneration := existing.batchGeneration
	if batchGeneration == nil {
		batchGeneration = w.newBatchGeneration()
	}
	batch := existing.batch
	if batch == nil {
		batch = w.newBatch(batchGeneration)
	}
	return taskStudioBatchCollaborators{
		batchGeneration: batchGeneration,
		batch:           batch,
	}
}

func buildTaskStudioMediaWiring(s *service) taskStudioMediaWiring {
	return taskStudioMediaWiring{
		imageGenerator:        resolveStudioImageGenerator(s),
		promptDiversifier:     resolveStudioPromptDiversifier(s),
		uploadStoreConfigured: resolveStudioUploadStore(s) != nil,
		uploadImages:          s.UploadImages,
	}
}

func buildStudioBatchGenerationWiring(s *service) studioBatchGenerationWiring {
	return studioBatchGenerationWiring{
		repo: resolveStudioBatchRepo(s),
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			return ExecuteStudioDesignBatch(ctx, s, input)
		},
	}
}

func resolveStudioSessionRepo(s *service) StudioSessionRepository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.studioDeps.sessionRepo, &s.mirrors.studioSessionRepo)
}

func resolveStudioBatchRepo(s *service) StudioBatchRepository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.studioDeps.batchRepo, &s.mirrors.studioBatchRepo)
}

func resolveStudioBatchRunRepo(s *service) StudioBatchRunRepository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.studioDeps.batchRunRepo, &s.mirrors.studioBatchRunRepo)
}

func resolveStudioPromptDiversifier(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.studioDeps.promptDiversifier, &s.mirrors.studioPromptDiversifier)
}

func resolveStudioImageGenerator(s *service) openaiclient.ImageGenerator {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.studioDeps.imageGenerator, &s.mirrors.studioImageGenerator)
}

func resolveStudioUploadStore(s *service) ImageUploadStore {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.studioDeps.uploadStore, &s.mirrors.uploadStore)
}
