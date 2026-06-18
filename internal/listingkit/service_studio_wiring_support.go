package listingkit

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

func resolveStudioPromptDiversifier(s *service) AIChatCompleter {
	if s == nil {
		return nil
	}
	return s.studioDeps.promptDiversifier
}

func resolveStudioImageGenerator(s *service) AIImageGenerator {
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
