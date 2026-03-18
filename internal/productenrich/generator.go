// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// JSONGenerator JSON 生成器接口
type JSONGenerator interface {
	// GenerateJSON 生成产品 JSON。
	// variantGen 为 nil 时跳过变体和规格生成（basic/minimal 策略）。
	// skipVariants 为 true 时只生成规格，跳过变体（basic 策略）。
	GenerateJSON(ctx context.Context, analysis *ProductAnalysis, variantGen VariantGenerator, skipVariants bool) (*ProductJSON, error)
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
