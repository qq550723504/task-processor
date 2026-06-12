package listingkit

import openaiclient "task-processor/internal/infra/clients/openai"

type studioDependencies struct {
	sessionRepo       StudioSessionRepository
	batchRepo         StudioBatchRepository
	batchRunRepo      StudioBatchRunRepository
	promptDiversifier openaiclient.ChatCompleter
	imageGenerator    openaiclient.ImageGenerator
	uploadStore       ImageUploadStore
}
