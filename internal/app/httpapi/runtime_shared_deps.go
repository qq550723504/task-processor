package httpapi

import (
	"context"
	"net/http"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
	"task-processor/internal/listingadmin"
	platformobservability "task-processor/internal/platform/observability"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

type traceRuntime interface {
	WrapHTTPHandler(http.Handler, string) http.Handler
	Shutdown(context.Context) error
}

type sharedRuntimeDeps struct {
	cfg                  *config.Config
	traceRuntime         traceRuntime
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

func traceRuntimeConfig(cfg *config.Config) platformobservability.Config {
	if cfg == nil {
		return platformobservability.Config{}
	}
	tracing := cfg.Observability.Tracing
	return platformobservability.Config{
		Enabled:     tracing.Enabled,
		ServiceName: tracing.ServiceName,
		Endpoint:    tracing.Endpoint,
		Insecure:    tracing.Insecure,
	}
}
