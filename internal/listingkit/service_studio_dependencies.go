package listingkit

type studioDependencies struct {
	sessionRepo       StudioSessionRepository
	batchRepo         StudioBatchRepository
	batchRunRepo      StudioBatchRunRepository
	promptDiversifier AIChatCompleter
	imageGenerator    AIImageGenerator
	backgroundRemover StudioBackgroundRemover
	uploadStore       ImageUploadStore
}
