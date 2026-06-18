package listingkit

import (
	"context"
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

type taskStudioMediaWiring struct {
	imageGenerator        AIImageGenerator
	promptDiversifier     AIChatCompleter
	uploadStoreConfigured bool
	uploadImages          func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
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
	return newTaskStudioSessionService(buildTaskStudioSessionServiceConfigWithWiring(buildTaskStudioSessionConfigWiring(w.service)))
}

func (w taskStudioSessionCollaboratorWiring) newBatchDraft() *taskStudioBatchDraftService {
	return newTaskStudioBatchDraftService(buildTaskStudioBatchDraftServiceConfigWithWiring(buildTaskStudioSessionConfigWiring(w.service)))
}

func (w taskStudioSessionCollaboratorWiring) newMedia() *taskStudioMediaService {
	return newTaskStudioMediaService(buildTaskStudioMediaServiceConfigWithWiring(buildTaskStudioMediaWiring(w.service)))
}

func (w taskStudioSessionCollaboratorWiring) resolve(existing taskStudioSessionCollaborators) taskStudioSessionCollaborators {
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

func buildTaskStudioMediaWiring(s *service) taskStudioMediaWiring {
	return taskStudioMediaWiring{
		imageGenerator:        resolveStudioImageGenerator(s),
		promptDiversifier:     resolveStudioPromptDiversifier(s),
		uploadStoreConfigured: resolveStudioUploadStore(s) != nil,
		uploadImages:          s.UploadImages,
	}
}

func buildTaskStudioSessionServiceConfigWithWiring(config taskStudioSessionConfigWiring) taskStudioSessionServiceConfig {
	return taskStudioSessionServiceConfig{
		repo:                     config.session.repo,
		runner:                   config.runner,
		asyncJobRunner:           config.asyncJobRunner,
		generationMetadataRunner: config.generationMetadataRunner,
		reviewTaskMetadataRunner: config.reviewTaskMetadataRunner,
		generalMetadataRunner:    config.generalMetadataRunner,
	}
}

func buildTaskStudioBatchDraftServiceConfigWithWiring(config taskStudioSessionConfigWiring) taskStudioBatchDraftServiceConfig {
	return taskStudioBatchDraftServiceConfig{
		repo:   config.session.repo,
		runner: config.batchDraftRunner,
	}
}

func buildTaskStudioMediaServiceConfigWithWiring(wiring taskStudioMediaWiring) taskStudioMediaServiceConfig {
	return taskStudioMediaServiceConfig{
		imageGenerator:        wiring.imageGenerator,
		promptDiversifier:     wiring.promptDiversifier,
		uploadStoreConfigured: wiring.uploadStoreConfigured,
		uploadImages:          wiring.uploadImages,
	}
}
