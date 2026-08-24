package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/watermark"
	"task-processor/internal/productenrich"
)

type RuntimeBuildInput struct {
	Logger               *logrus.Logger
	Config               *config.Config
	LLMManager           productenrich.LLMManager
	OpenAIManager        *openaiclient.Manager
	AICredentialResolver openaiclient.ClientConfigResolver
	AIInvocationRecorder aicapability.InvocationRecorder
	InputParser          productenrich.InputParser
	Understanding        productenrich.ProductUnderstanding
	ImageWorkDir         string
}

type BuildModuleInput struct {
	Logger               *logrus.Logger
	LLMManager           productenrich.LLMManager
	OpenAIManager        *openaiclient.Manager
	AICredentialResolver openaiclient.ClientConfigResolver
	AIInvocationRecorder aicapability.InvocationRecorder
	InputParser          productenrich.InputParser
	Understanding        productenrich.ProductUnderstanding
	ImageWorkDir         string
	Options              productImageRuntimeOptions
}

type productImageRuntimeOptions struct {
	database                *config.DatabaseConfig
	watermark               *watermark.Config
	cleanupTemporaryFiles   bool
	reuseExistingAssets     bool
	requireAIIdentity       bool
	workerConcurrency       int
	workerBufferSize        int
	assetPublisher          assetPublisherOptions
	modelProvider           modelProviderOptions
	imagePipelineComponents imagePipelineComponentOptions
	sceneGovernance         sceneGovernanceOptions
}

func newProductImageRuntimeOptions(cfg *config.Config) productImageRuntimeOptions {
	if cfg == nil {
		return productImageRuntimeOptions{}
	}
	return productImageRuntimeOptions{
		database:                cfg.Database,
		watermark:               cfg.Watermark,
		cleanupTemporaryFiles:   cfg.ProductImage.Lifecycle.CleanupTemporaryFiles,
		reuseExistingAssets:     cfg.ProductImage.Lifecycle.ReuseExistingAssets,
		// Governed model stages enforce identity for allowlisted tenants. Task
		// creation remains compatible with legacy callers that never enter that path.
		requireAIIdentity:       false,
		workerConcurrency:       cfg.Worker.Concurrency,
		workerBufferSize:        cfg.Worker.BufferSize,
		assetPublisher:          newAssetPublisherOptions(cfg),
		modelProvider:           newModelProviderOptions(cfg),
		imagePipelineComponents: newImagePipelineComponentOptions(cfg),
		sceneGovernance:         newSceneGovernanceOptions(cfg),
	}
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		Logger:               input.Logger,
		LLMManager:           input.LLMManager,
		OpenAIManager:        input.OpenAIManager,
		AICredentialResolver: input.AICredentialResolver,
		AIInvocationRecorder: input.AIInvocationRecorder,
		InputParser:          input.InputParser,
		Understanding:        input.Understanding,
		ImageWorkDir:         input.ImageWorkDir,
		Options:              newProductImageRuntimeOptions(input.Config),
	})
}
