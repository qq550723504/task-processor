package httpapi

import (
	"context"
	"net/http"

	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
	"task-processor/internal/listingadmin"
	platformobservability "task-processor/internal/platform/observability"
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
	storeAPI             listingadmin.StoreAPI
	productCatalogDB     *gorm.DB
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
