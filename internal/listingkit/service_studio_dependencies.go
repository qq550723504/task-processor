package listingkit

type studioDependencies struct {
	sessionRepo       StudioSessionRepository
	batchRepo         StudioBatchRepository
	batchRunRepo      StudioBatchRunRepository
	batchTaskLinkRepo StudioBatchTaskLinkRepository
	promptDiversifier AIChatCompleter
	imageGenerator    AIImageGenerator
	uploadStore       ImageUploadStore
}
