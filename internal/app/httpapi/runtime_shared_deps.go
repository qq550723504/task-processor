package httpapi

import (
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
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
	storeAPI          listingadmin.StoreAPI
}
