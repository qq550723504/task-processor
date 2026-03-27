package enrich

import (
	"fmt"

	productenrich "task-processor/internal/productenrich"

	"github.com/sirupsen/logrus"
)

type jsonGenerator struct {
	logger     *logrus.Logger
	llmManager productenrich.LLMManager
}

func NewJSONGenerator(logger *logrus.Logger, llmManager productenrich.LLMManager) (productenrich.JSONGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}

	return &jsonGenerator{
		logger:     logger,
		llmManager: llmManager,
	}, nil
}
