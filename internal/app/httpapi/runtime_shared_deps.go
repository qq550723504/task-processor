package httpapi

import (
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	"task-processor/internal/prompt"
)

type sharedRuntimeDeps struct {
	cfg               *config.Config
	closers           []func() error
	openaiMgr         *openaiclient.Manager
	aiCredentialStore *openaiclient.GormCredentialResolver
	tenantPromptStore prompt.TenantPromptStore
	llmMgr            productenrich.LLMManager
	inputParser       productenrich.InputParser
	understanding     productenrich.ProductUnderstanding
	imageWorkDir      string
	sharedResources   *bootstrapresources.SharedResources
}
