// Package pricing 提供决策配置
package pricing

// DecisionConfig 决策配置
type DecisionConfig struct {
	DefaultTargetMargin float64 // 默认目标利润率
	DefaultMinMargin    float64 // 默认最小利润率
	MaxRetries          int     // 最大重试次数
	UseAmazonPrice      bool    // 是否使用Amazon价格
}

// DefaultDecisionConfig 返回默认决策配置
func DefaultDecisionConfig() *DecisionConfig {
	return &DecisionConfig{
		DefaultTargetMargin: 1.5,
		DefaultMinMargin:    1.5,
		MaxRetries:          3,
		UseAmazonPrice:      true,
	}
}

// NewDecisionConfigFromPlatform 从平台配置创建决策配置
func NewDecisionConfigFromPlatform(platformConfig interface{}) *DecisionConfig {
	config := DefaultDecisionConfig()

	// 这里可以根据platformConfig设置具体值
	// 暂时使用默认配置

	return config
}
