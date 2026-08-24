package enrich

import (
	"fmt"

	productenrich "task-processor/internal/productenrich"

	"github.com/sirupsen/logrus"
)

type jsonGenerator struct {
	logger        *logrus.Logger
	llmManager    productenrich.LLMManager
	textGenerator TextGenerator
}

func NewJSONGenerator(logger *logrus.Logger, llmManager productenrich.LLMManager) (productenrich.JSONGenerator, error) {
	return NewJSONGeneratorWithTextGenerator(logger, llmManager, nil)
}

// NewJSONGeneratorWithTextGenerator keeps JSON assembly in ProductEnrich while
// allowing its primary model call to use a governed text capability.
func NewJSONGeneratorWithTextGenerator(logger *logrus.Logger, llmManager productenrich.LLMManager, textGenerator TextGenerator) (productenrich.JSONGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}

	return &jsonGenerator{
		logger: logger, llmManager: llmManager, textGenerator: textGenerator,
	}, nil
}
