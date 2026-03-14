// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"fmt"

	"task-processor/internal/domain/productjson"

	"github.com/sirupsen/logrus"
)

// JSONGenerator JSON 生成器接口
type JSONGenerator interface {
	// GenerateJSON 生成产品 JSON
	GenerateJSON(ctx context.Context, analysis *productjson.ProductAnalysis, variantGen VariantGenerator) (*productjson.ProductJSON, error)
}

// jsonGenerator JSON 生成器实现
type jsonGenerator struct {
	logger     *logrus.Logger
	llmManager LLMManager
}

// NewJSONGenerator 创建新的 JSON 生成器
func NewJSONGenerator(logger *logrus.Logger, llmManager LLMManager) (JSONGenerator, error) {
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
