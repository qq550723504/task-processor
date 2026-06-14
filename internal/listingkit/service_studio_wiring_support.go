package listingkit

import (
	"context"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type taskStudioSessionRepoWiring struct {
	repo StudioSessionRepository
}

type taskStudioBatchServiceWiring struct {
	repo               StudioBatchRepository
	studioSessionRepo  StudioSessionRepository
	generator          *studioBatchGenerationService
	createGenerateTask func(context.Context, *GenerateRequest) (*Task, error)
	getTask            func(context.Context, string) (*Task, error)
	detailRunner       *listingStudioBatchDetailRunner
	reviewRunner       *listingStudioBatchReviewRunner
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

func buildTaskStudioSessionRepoWiring(s *service) taskStudioSessionRepoWiring {
	return taskStudioSessionRepoWiring{
		repo: resolveStudioSessionRepo(s),
	}
}

func buildTaskStudioBatchRunRepoWiring(s *service) taskStudioBatchRunRepoWiring {
	return taskStudioBatchRunRepoWiring{
		repo:              resolveStudioBatchRunRepo(s),
		studioSessionRepo: resolveStudioSessionRepo(s),
	}
}

func buildTaskStudioBatchServiceWiring(s *service) taskStudioBatchServiceWiring {
	if s == nil {
		return taskStudioBatchServiceWiring{}
	}
	repository := buildServiceRepositoryWiring(s)
	return taskStudioBatchServiceWiring{
		repo:               resolveStudioBatchRepo(s),
		studioSessionRepo:  resolveStudioSessionRepo(s),
		generator:          s.studioBatchGenerationOrDefault(),
		createGenerateTask: s.CreateGenerateTask,
		getTask:            repository.getTask,
		detailRunner: newListingStudioBatchDetailService(
			resolveStudioBatchRepo(s),
			resolveStudioSessionRepo(s),
			func(ctx context.Context, batchID string) error {
				return ensureStudioBatchGenerationGraphForResume(ctx, resolveStudioBatchRepo(s), resolveStudioSessionRepo(s), time.Now, batchID)
			},
		),
		reviewRunner: newListingStudioBatchReviewService(
			resolveStudioBatchRepo(s),
			func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
				return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
			},
			time.Now,
		),
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
