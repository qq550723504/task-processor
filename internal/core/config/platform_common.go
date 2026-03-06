// Package config 提供平台通用配置类型
package config

// CostCalculationConfig 成本计算配置（平台通用）
// 用于TEMU、SHEIN等平台的成本计算
type CostCalculationConfig struct {
	FixedCostAmount    float64 `json:"fixed_cost_amount" yaml:"fixed_cost_amount"`     // 固定成本金额
	FixedCostPercent   float64 `json:"fixed_cost_percent" yaml:"fixed_cost_percent"`   // 固定成本百分比
	ShippingCost       float64 `json:"shipping_cost" yaml:"shipping_cost"`             // 运费成本
	ProcessingFee      float64 `json:"processing_fee" yaml:"processing_fee"`           // 处理费用
	PlatformCommission float64 `json:"platform_commission" yaml:"platform_commission"` // 平台佣金百分比
}

// DefaultCostCalculationConfig 返回默认成本计算配置
func DefaultCostCalculationConfig() *CostCalculationConfig {
	return &CostCalculationConfig{
		FixedCostAmount:    0,
		FixedCostPercent:   0,
		ShippingCost:       0,
		ProcessingFee:      0,
		PlatformCommission: 0,
	}
}

// CalculateTotalCost 计算总成本
func (c *CostCalculationConfig) CalculateTotalCost(basePrice float64) float64 {
	totalCost := basePrice

	// 添加固定金额成本
	totalCost += c.FixedCostAmount

	// 添加固定百分比成本
	if c.FixedCostPercent > 0 {
		totalCost += basePrice * (c.FixedCostPercent / 100.0)
	}

	// 添加运费成本
	totalCost += c.ShippingCost

	// 添加处理费用
	totalCost += c.ProcessingFee

	// 添加平台佣金
	if c.PlatformCommission > 0 {
		totalCost += basePrice * (c.PlatformCommission / 100.0)
	}

	return totalCost
}

// SensitiveWordsConfig 敏感词配置（平台通用）
// 用于TEMU、SHEIN等平台的敏感词过滤
type SensitiveWordsConfig struct {
	StaticWords  map[string][]string `json:"static_words" yaml:"static_words"`   // 按语言分类的静态敏感词
	DynamicWords map[string][]string `json:"dynamic_words" yaml:"dynamic_words"` // 按语言分类的动态敏感词
	LastUpdated  string              `json:"last_updated" yaml:"last_updated"`   // 最后更新时间
	Version      string              `json:"version" yaml:"version"`             // 配置版本
	Platform     string              `json:"platform" yaml:"platform"`           // 平台名称
}

// DefaultSensitiveWordsConfig 返回默认敏感词配置
func DefaultSensitiveWordsConfig() *SensitiveWordsConfig {
	return &SensitiveWordsConfig{
		StaticWords:  make(map[string][]string),
		DynamicWords: make(map[string][]string),
	}
}

// GetWordsForLanguage 获取指定语言的所有敏感词
func (c *SensitiveWordsConfig) GetWordsForLanguage(language string) []string {
	var words []string

	if staticWords, ok := c.StaticWords[language]; ok {
		words = append(words, staticWords...)
	}

	if dynamicWords, ok := c.DynamicWords[language]; ok {
		words = append(words, dynamicWords...)
	}

	return words
}

// AddStaticWords 添加静态敏感词
func (c *SensitiveWordsConfig) AddStaticWords(language string, words []string) {
	if c.StaticWords == nil {
		c.StaticWords = make(map[string][]string)
	}
	c.StaticWords[language] = append(c.StaticWords[language], words...)
}

// AddDynamicWords 添加动态敏感词
func (c *SensitiveWordsConfig) AddDynamicWords(language string, words []string) {
	if c.DynamicWords == nil {
		c.DynamicWords = make(map[string][]string)
	}
	c.DynamicWords[language] = append(c.DynamicWords[language], words...)
}
