package httpapi

import (
	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

type sharedRuntimeDeps struct {
	cfg                  *config.Config
	featureFlags         BoolEvaluator
	closers              []func() error
	openaiMgr            *openaiclient.Manager
	aiCredentialStore    *openaiclient.GormCredentialResolver
	aiInvocationRecorder aicapability.InvocationRecorder
	aiAsyncJobStore      aicapability.AsyncJobBindingStore
	tenantPromptStore    prompt.TenantPromptStore
	llmMgr               productenrich.LLMManager
	inputParser          productenrich.InputParser
	understanding        productenrich.ProductUnderstanding
	contentGenerator     productenrichenrich.TextGenerator
	specsGenerator       productenrichenrich.TextGenerator
	variantsGenerator    productenrichenrich.TextGenerator
	fusionGenerator      productenrichenrich.TextGenerator
	scoringTextGenerator productenrichenrich.TextGenerator
	scoringImageAnalyzer productenrichenrich.ImageAnalyzer
	imageWorkDir         string
	sourceImageFetcher   productimage.SourceImageFetcher
	storeAPI             listingadmin.StoreAPI
}
