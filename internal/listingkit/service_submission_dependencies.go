package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type submissionDependencies struct {
	storeProfileRepo            StoreProfileRepository
	sheinProductAPIBuilder      sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder        sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder    sheinpub.TranslateAPIBuilder
	sheinPublishWorkflowClient  SheinPublishWorkflowClient
	sheinPublishWorkflowEnabled bool
}
