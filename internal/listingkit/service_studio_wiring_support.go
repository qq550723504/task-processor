package listingkit

import (
	"context"

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
	if s.studioDeps.sessionRepo != nil {
		s.studioSessionRepo = s.studioDeps.sessionRepo
		return s.studioDeps.sessionRepo
	}
	s.studioDeps.sessionRepo = s.studioSessionRepo
	return s.studioSessionRepo
}

func resolveStudioBatchRepo(s *service) StudioBatchRepository {
	if s == nil {
		return nil
	}
	if s.studioDeps.batchRepo != nil {
		s.studioBatchRepo = s.studioDeps.batchRepo
		return s.studioDeps.batchRepo
	}
	s.studioDeps.batchRepo = s.studioBatchRepo
	return s.studioBatchRepo
}

func resolveStudioBatchRunRepo(s *service) StudioBatchRunRepository {
	if s == nil {
		return nil
	}
	if s.studioDeps.batchRunRepo != nil {
		s.studioBatchRunRepo = s.studioDeps.batchRunRepo
		return s.studioDeps.batchRunRepo
	}
	s.studioDeps.batchRunRepo = s.studioBatchRunRepo
	return s.studioBatchRunRepo
}

func resolveStudioPromptDiversifier(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	if s.studioDeps.promptDiversifier != nil {
		s.studioPromptDiversifier = s.studioDeps.promptDiversifier
		return s.studioDeps.promptDiversifier
	}
	s.studioDeps.promptDiversifier = s.studioPromptDiversifier
	return s.studioPromptDiversifier
}

func resolveStudioImageGenerator(s *service) openaiclient.ImageGenerator {
	if s == nil {
		return nil
	}
	if s.studioDeps.imageGenerator != nil {
		s.studioImageGenerator = s.studioDeps.imageGenerator
		return s.studioDeps.imageGenerator
	}
	s.studioDeps.imageGenerator = s.studioImageGenerator
	return s.studioImageGenerator
}

func resolveStudioUploadStore(s *service) ImageUploadStore {
	if s == nil {
		return nil
	}
	if s.studioDeps.uploadStore != nil {
		s.uploadStore = s.studioDeps.uploadStore
		return s.studioDeps.uploadStore
	}
	s.studioDeps.uploadStore = s.uploadStore
	return s.uploadStore
}
