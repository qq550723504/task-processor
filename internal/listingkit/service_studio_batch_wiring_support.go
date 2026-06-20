package listingkit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
)

type taskStudioBatchServiceWiring struct {
	repo               StudioBatchRepository
	batchRunRepo       StudioBatchRunRepository
	batchTaskLinkRepo  StudioBatchTaskLinkRepository
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
	repo        StudioBatchRepository
	execute     func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error)
	submitAsync func(context.Context, StudioBatchGenerateExecutionInput) (*studioBatchAsyncSubmitOutput, error)
	queryAsync  func(context.Context, StudioBatchGenerateExecutionInput, string) (*studioBatchAsyncQueryOutput, error)
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
		batchRunRepo:       resolveStudioBatchRunRepo(s),
		batchTaskLinkRepo:  resolveStudioBatchTaskLinkRepo(s),
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
				repo:         repo,
				batchRunRepo: resolveStudioBatchRunRepo(s),
				currentTime:  time.Now,
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
		submitAsync: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*studioBatchAsyncSubmitOutput, error) {
			logStudioBatchSubmitAsyncDiagnostic(ctx, input)
			result, err := s.SubmitStudioDesignsAsync(ctx, input.Request)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, nil
			}
			output := &studioBatchAsyncSubmitOutput{
				Submit:   result.Submit,
				Response: result.Response,
			}
			if result.Response != nil {
				payload, marshalErr := json.Marshal(result.Response)
				if marshalErr != nil {
					return nil, marshalErr
				}
				output.ResultPayload = string(payload)
			}
			return output, nil
		},
		queryAsync: func(ctx context.Context, input StudioBatchGenerateExecutionInput, jobID string) (*studioBatchAsyncQueryOutput, error) {
			result, err := s.QueryStudioDesignsAsync(ctx, input.Request, jobID)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, nil
			}
			output := &studioBatchAsyncQueryOutput{
				Result:   result.Result,
				Response: result.Response,
			}
			if result.Response != nil {
				payload, marshalErr := json.Marshal(result.Response)
				if marshalErr != nil {
					return nil, marshalErr
				}
				output.ResultPayload = string(payload)
			} else if result.Result != nil {
				output.ResultPayload = result.Result.RawResultResponse
			}
			return output, nil
		},
	}
}

func logStudioBatchSubmitAsyncDiagnostic(ctx context.Context, input StudioBatchGenerateExecutionInput) {
	fields := logrus.Fields{
		"batch_id":   input.BatchID,
		"item_id":    input.ItemID,
		"attempt_id": input.AttemptID,
	}
	if input.Request != nil {
		fields["image_model"] = input.Request.ImageModel
		fields["count"] = input.Request.Count
		fields["reference_count"] = len(input.Request.ProductReferenceImageURLs)
	}
	if deadline, ok := ctx.Deadline(); ok {
		fields["ctx_deadline"] = deadline.UTC().Format(time.RFC3339Nano)
		fields["ctx_deadline_remaining_ms"] = time.Until(deadline).Milliseconds()
	} else {
		fields["ctx_deadline"] = "none"
	}
	logrus.WithFields(fields).Info("listingkit studio batch async submit diagnostic")
}

func buildStudioBatchGenerationServiceConfigWithWiring(wiring studioBatchGenerationWiring) studioBatchGenerationServiceConfig {
	return studioBatchGenerationServiceConfig{
		repo:        wiring.repo,
		execute:     wiring.execute,
		submitAsync: wiring.submitAsync,
		queryAsync:  wiring.queryAsync,
	}
}

func buildTaskStudioBatchServiceConfigWithCollaborators(
	config taskStudioBatchServiceConfigWiring,
) taskStudioBatchServiceConfig {
	return taskStudioBatchServiceConfig{
		repo:               config.batch.repo,
		batchRunRepo:       config.batch.batchRunRepo,
		batchTaskLinkRepo:  config.batch.batchTaskLinkRepo,
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
