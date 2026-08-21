package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
)

type RuntimeBuildInput struct {
	Logger        *logrus.Logger
	Config        *config.Config
	LLMManager    productenrich.LLMManager
	TextGenerator productenrichenrich.TextGenerator
	InputParser   productenrich.InputParser
	Understanding productenrich.ProductUnderstanding
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		Config:        input.Config,
		Logger:        input.Logger,
		LLMManager:    input.LLMManager,
		TextGenerator: input.TextGenerator,
		InputParser:   input.InputParser,
		Understanding: input.Understanding,
	})
}
