// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// StrategySelector 策略选择器接口
type StrategySelector interface {
	// SelectStrategy 根据质量评分选择处理策略
	SelectStrategy(ctx context.Context, qualityScore float64) (ProcessingStrategy, error)
	// GetStrategyDetails 获取策略详细信息
	GetStrategyDetails(strategy ProcessingStrategy) (*StrategyDetails, error)
}

// strategySelector 策略选择器实现
type strategySelector struct {
	metrics          MetricsCollector
	fullThreshold    float64
	basicThreshold   float64
	minimalThreshold float64
}

// StrategySelectorConfig 策略选择器配置
type StrategySelectorConfig struct {
	FullThreshold    float64          // 完整处理策略阈值
	BasicThreshold   float64          // 基础处理策略阈值
	MinimalThreshold float64          // 最小处理策略阈值
	Metrics          MetricsCollector // 指标收集器
}

// NewStrategySelector 创建新的策略选择器
func NewStrategySelector(config *StrategySelectorConfig) StrategySelector {
	if config == nil {
		config = &StrategySelectorConfig{
			FullThreshold:    80.0,
			BasicThreshold:   60.0,
			MinimalThreshold: 50.0,
		}
	}

	return &strategySelector{
		metrics:          config.Metrics,
		fullThreshold:    config.FullThreshold,
		basicThreshold:   config.BasicThreshold,
		minimalThreshold: config.MinimalThreshold,
	}
}

// SelectStrategy 根据质量评分选择处理策略
func (s *strategySelector) SelectStrategy(ctx context.Context, qualityScore float64) (ProcessingStrategy, error) {
	var strategy ProcessingStrategy

	// 使用配置的阈值进行策略选择
	if qualityScore >= s.fullThreshold {
		strategy = StrategyFull
	} else if qualityScore >= s.basicThreshold {
		strategy = StrategyBasic
	} else if qualityScore >= s.minimalThreshold {
		strategy = StrategyMinimal
	} else {
		strategy = StrategyReject
	}

	// 记录策略选择指标
	if s.metrics != nil {
		s.metrics.RecordCacheOperation("strategy", string(strategy))
	}

	logger.GetGlobalLogger("productenrich/strategy.go").WithFields(logrus.Fields{
		"quality_score":     qualityScore,
		"strategy":          string(strategy),
		"full_threshold":    s.fullThreshold,
		"basic_threshold":   s.basicThreshold,
		"minimal_threshold": s.minimalThreshold,
	}).Info("processing strategy selected")

	return strategy, nil
}

// GetStrategyDetails 获取策略详细信息
func (s *strategySelector) GetStrategyDetails(strategy ProcessingStrategy) (*StrategyDetails, error) {
	switch strategy {
	case StrategyFull:
		return &StrategyDetails{
			Strategy:    StrategyFull,
			Description: "完整处理：执行所有处理步骤，生成完整的产品信息",
			EnabledSteps: []string{
				"图片分析",
				"文本提取",
				"多模态融合",
				"规格生成",
				"变体生成",
				"SEO 优化",
			},
			DisabledSteps:     []string{},
			ExpectedQuality:   "高质量",
			EstimatedCost:     "高",
			EstimatedDuration: "60-120秒",
		}, nil

	case StrategyBasic:
		return &StrategyDetails{
			Strategy:    StrategyBasic,
			Description: "基础处理：跳过复杂的变体生成和详细规格提取",
			EnabledSteps: []string{
				"图片分析",
				"文本提取",
				"多模态融合",
				"基础规格生成",
			},
			DisabledSteps: []string{
				"详细规格提取",
				"变体生成",
			},
			ExpectedQuality:   "中等质量",
			EstimatedCost:     "中等",
			EstimatedDuration: "30-60秒",
		}, nil

	case StrategyMinimal:
		return &StrategyDetails{
			Strategy:    StrategyMinimal,
			Description: "最小处理：仅生成基础产品信息，不生成规格和变体",
			EnabledSteps: []string{
				"基础信息提取",
				"简单分类",
			},
			DisabledSteps: []string{
				"图片深度分析",
				"规格生成",
				"变体生成",
				"SEO 优化",
			},
			ExpectedQuality:   "基础质量",
			EstimatedCost:     "低",
			EstimatedDuration: "10-30秒",
		}, nil

	case StrategyReject:
		return &StrategyDetails{
			Strategy:          StrategyReject,
			Description:       "拒绝处理：数据质量不足，无法生成有效的产品信息",
			EnabledSteps:      []string{},
			DisabledSteps:     []string{"所有处理步骤"},
			ExpectedQuality:   "无法生成",
			EstimatedCost:     "无",
			EstimatedDuration: "立即返回",
		}, nil

	default:
		logger.GetGlobalLogger("productenrich/strategy.go").WithField("strategy", strategy).Error("unknown strategy")
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}
}
