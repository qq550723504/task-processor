package listingkit

import (
	"context"
	"time"
)

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

type studioBatchGenerationWiring struct {
	repo    StudioBatchRepository
	execute func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error)
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
	return newStudioBatchGenerationService(buildStudioBatchGenerationServiceConfigWithWiring(buildStudioBatchGenerationWiring(w.service)))
}

func (w taskStudioBatchCollaboratorWiring) newBatch(batchGeneration *studioBatchGenerationService) *taskStudioBatchService {
	return newTaskStudioBatchService(buildTaskStudioBatchServiceConfigWithCollaborators(
		buildTaskStudioBatchServiceConfigWiringWithGenerator(w.service, batchGeneration),
	))
}

func (w taskStudioBatchCollaboratorWiring) resolve(existing taskStudioBatchCollaborators) taskStudioBatchCollaborators {
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

func buildStudioBatchGenerationWiring(s *service) studioBatchGenerationWiring {
	return studioBatchGenerationWiring{
		repo: resolveStudioBatchRepo(s),
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			return ExecuteStudioDesignBatch(ctx, s, input)
		},
	}
}

func buildStudioBatchGenerationServiceConfigWithWiring(wiring studioBatchGenerationWiring) studioBatchGenerationServiceConfig {
	return studioBatchGenerationServiceConfig{
		repo:    wiring.repo,
		execute: wiring.execute,
	}
}

func buildTaskStudioBatchServiceConfigWithCollaborators(
	config taskStudioBatchServiceConfigWiring,
) taskStudioBatchServiceConfig {
	return taskStudioBatchServiceConfig{
		repo:               config.batch.repo,
		studioSessionRepo:  config.batch.studioSessionRepo,
		generator:          config.batch.generator,
		createGenerateTask: config.batch.createGenerateTask,
		getTask:            config.batch.getTask,
		detailRunner:       config.detailRunner,
		reviewRunner:       config.reviewRunner,
		retryRunner:        config.retryRunner,
		taskPrepareRunner:  config.taskPrepare,
		taskResumeRunner:   config.taskResume,
	}
}
