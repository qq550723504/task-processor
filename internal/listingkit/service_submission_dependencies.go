package listingkit

import openaiclient "task-processor/internal/infra/clients/openai"
import sheinpub "task-processor/internal/publishing/shein"

type submissionDependencies struct {
	storeProfileRepo            StoreProfileRepository
	sheinProductAPIBuilder      sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder        sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder    sheinpub.TranslateAPIBuilder
	sheinContentOptimizer       openaiclient.ChatCompleter
	sheinPublishWorkflowClient  SheinPublishWorkflowClient
	sheinPublishWorkflowEnabled bool
}
