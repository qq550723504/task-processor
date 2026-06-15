package listingkit

import (
	openaiclient "task-processor/internal/infra/clients/openai"
)

func resolveStudioSessionRepo(s *service) StudioSessionRepository {
	if s == nil {
		return nil
	}
	return s.studioDeps.sessionRepo
}

func resolveStudioBatchRepo(s *service) StudioBatchRepository {
	if s == nil {
		return nil
	}
	return s.studioDeps.batchRepo
}

func resolveStudioBatchRunRepo(s *service) StudioBatchRunRepository {
	if s == nil {
		return nil
	}
	return s.studioDeps.batchRunRepo
}

func resolveStudioPromptDiversifier(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	return s.studioDeps.promptDiversifier
}

func resolveStudioImageGenerator(s *service) openaiclient.ImageGenerator {
	if s == nil {
		return nil
	}
	return s.studioDeps.imageGenerator
}

func resolveStudioUploadStore(s *service) ImageUploadStore {
	if s == nil {
		return nil
	}
	return s.studioDeps.uploadStore
}
