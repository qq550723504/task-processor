package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
)

type RuntimeBuildInput struct {
	Logger        *logrus.Logger
	Config        *config.Config
	LLMManager    productenrich.LLMManager
	OpenAIManager *openaiclient.Manager
	InputParser   productenrich.InputParser
	Understanding productenrich.ProductUnderstanding
	ImageWorkDir  string
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		Config:        input.Config,
		Logger:        input.Logger,
		LLMManager:    input.LLMManager,
		OpenAIManager: input.OpenAIManager,
		InputParser:   input.InputParser,
		Understanding: input.Understanding,
		ImageWorkDir:  input.ImageWorkDir,
	})
}
