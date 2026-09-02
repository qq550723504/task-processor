package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type taskStudioBatchServiceWiring struct {
	repo                   StudioBatchRepository
	batchRunRepo           StudioBatchRunRepository
	studioSessionRepo      StudioSessionRepository
	generator              *studioBatchGenerationService
	retryBackgroundRemoval func(context.Context, string, string) (*studioBackgroundRemovalMaterialization, error)
	ensureGraph            func(context.Context, string) error
	loadDetail             func(context.Context, string) (*StudioBatchDetail, error)
	resetRetryItems        func(context.Context, []StudioBatchItemRecord) error
	currentTime            func() time.Time
}

type taskStudioBatchServiceConfigWiring struct {
	batch        taskStudioBatchServiceWiring
	detailRunner *listingStudioBatchDetailRunner
	reviewRunner *listingStudioBatchReviewRunner
	retryRunner  *listingStudioBatchRetryPrepareRunner
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
	repo := resolveStudioBatchRepo(s)
	studioSessionRepo := resolveStudioSessionRepo(s)
	return taskStudioBatchServiceWiring{
		repo:              repo,
		batchRunRepo:      resolveStudioBatchRunRepo(s),
		studioSessionRepo: studioSessionRepo,
		generator:         generator,
		retryBackgroundRemoval: func(ctx context.Context, sourceURL string, filename string) (*studioBackgroundRemovalMaterialization, error) {
			media := s.taskStudioMediaOrDefault()
			if media == nil {
				return nil, fmt.Errorf("studio media service is not configured")
			}
			return media.retryStudioBackgroundRemoval(ctx, sourceURL, filename)
		},
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
	return taskStudioBatchServiceConfigWiring{
		batch:        batch,
		detailRunner: batch.newDetailRunner(),
		reviewRunner: batch.newReviewRunner(),
		retryRunner:  newListingStudioBatchRetryPrepareService(batch.repo, batch.loadDetail, batch.resetRetryItems),
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
		repo:                   config.batch.repo,
		batchRunRepo:           config.batch.batchRunRepo,
		studioSessionRepo:      config.batch.studioSessionRepo,
		generator:              config.batch.generator,
		retryBackgroundRemoval: config.batch.retryBackgroundRemoval,
		detailRunner:           config.detailRunner,
		reviewRunner:           config.reviewRunner,
		retryRunner:            config.retryRunner,
	}
}
