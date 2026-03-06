package config

import (
	"testing"
)

// TestDefaultCostCalculationConfig 测试默认成本计算配置
func TestDefaultCostCalculationConfig(t *testing.T) {
	config := DefaultCostCalculationConfig()

	if config == nil {
		t.Fatal("DefaultCostCalculationConfig 返回 nil")
	}

	// 验证所有默认值都是0
	if config.FixedCostAmount != 0 {
		t.Errorf("FixedCostAmount = %v, 期望 0", config.FixedCostAmount)
	}

	if config.FixedCostPercent != 0 {
		t.Errorf("FixedCostPercent = %v, 期望 0", config.FixedCostPercent)
	}

	if config.ShippingCost != 0 {
		t.Errorf("ShippingCost = %v, 期望 0", config.ShippingCost)
	}

	if config.ProcessingFee != 0 {
		t.Errorf("ProcessingFee = %v, 期望 0", config.ProcessingFee)
	}

	if config.PlatformCommission != 0 {
		t.Errorf("PlatformCommission = %v, 期望 0", config.PlatformCommission)
	}
}

// TestCalculateTotalCost 测试总成本计算
func TestCalculateTotalCost(t *testing.T) {
	tests := []struct {
		name      string
		config    *CostCalculationConfig
		basePrice float64
		expected  float64
	}{
		{
			name:      "默认配置（无额外成本）",
			config:    DefaultCostCalculationConfig(),
			basePrice: 100.0,
			expected:  100.0,
		},
		{
			name: "仅固定金额成本",
			config: &CostCalculationConfig{
				FixedCostAmount: 5.0,
			},
			basePrice: 100.0,
			expected:  105.0,
		},
		{
			name: "仅固定百分比成本（10%）",
			config: &CostCalculationConfig{
				FixedCostPercent: 10.0,
			},
			basePrice: 100.0,
			expected:  110.0,
		},
		{
			name: "仅运费成本",
			config: &CostCalculationConfig{
				ShippingCost: 8.0,
			},
			basePrice: 100.0,
			expected:  108.0,
		},
		{
			name: "仅处理费用",
			config: &CostCalculationConfig{
				ProcessingFee: 3.0,
			},
			basePrice: 100.0,
			expected:  103.0,
		},
		{
			name: "仅平台佣金（5%）",
			config: &CostCalculationConfig{
				PlatformCommission: 5.0,
			},
			basePrice: 100.0,
			expected:  105.0,
		},
		{
			name: "所有成本组合",
			config: &CostCalculationConfig{
				FixedCostAmount:    5.0,  // +5
				FixedCostPercent:   10.0, // +10 (10% of 100)
				ShippingCost:       8.0,  // +8
				ProcessingFee:      3.0,  // +3
				PlatformCommission: 5.0,  // +5 (5% of 100)
			},
			basePrice: 100.0,
			expected:  131.0, // 100 + 5 + 10 + 8 + 3 + 5
		},
		{
			name: "零基础价格",
			config: &CostCalculationConfig{
				FixedCostAmount: 5.0,
				ShippingCost:    8.0,
			},
			basePrice: 0,
			expected:  13.0, // 0 + 5 + 8
		},
		{
			name: "小数基础价格",
			config: &CostCalculationConfig{
				FixedCostPercent: 10.0,
			},
			basePrice: 99.99,
			expected:  109.989, // 99.99 + 9.999
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.CalculateTotalCost(tt.basePrice)
			// 使用小的误差范围比较浮点数
			if diff := result - tt.expected; diff < -0.001 || diff > 0.001 {
				t.Errorf("CalculateTotalCost() = %v, 期望 %v", result, tt.expected)
			}
		})
	}
}

// TestDefaultSensitiveWordsConfig 测试默认敏感词配置
func TestDefaultSensitiveWordsConfig(t *testing.T) {
	config := DefaultSensitiveWordsConfig()

	if config == nil {
		t.Fatal("DefaultSensitiveWordsConfig 返回 nil")
	}

	// 验证map已初始化
	if config.StaticWords == nil {
		t.Error("StaticWords 应该被初始化")
	}

	if config.DynamicWords == nil {
		t.Error("DynamicWords 应该被初始化")
	}

	// 验证map为空
	if len(config.StaticWords) != 0 {
		t.Errorf("StaticWords 长度 = %d, 期望 0", len(config.StaticWords))
	}

	if len(config.DynamicWords) != 0 {
		t.Errorf("DynamicWords 长度 = %d, 期望 0", len(config.DynamicWords))
	}
}

// TestGetWordsForLanguage 测试获取指定语言的敏感词
func TestGetWordsForLanguage(t *testing.T) {
	config := &SensitiveWordsConfig{
		StaticWords: map[string][]string{
			"en": {"bad", "worse"},
			"zh": {"敏感", "违禁"},
		},
		DynamicWords: map[string][]string{
			"en": {"terrible"},
			"zh": {"禁止"},
		},
	}

	tests := []struct {
		name     string
		language string
		expected int // 期望的词数
	}{
		{
			name:     "英语敏感词",
			language: "en",
			expected: 3, // bad, worse, terrible
		},
		{
			name:     "中文敏感词",
			language: "zh",
			expected: 3, // 敏感, 违禁, 禁止
		},
		{
			name:     "不存在的语言",
			language: "fr",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			words := config.GetWordsForLanguage(tt.language)
			if len(words) != tt.expected {
				t.Errorf("GetWordsForLanguage(%q) 返回 %d 个词, 期望 %d", tt.language, len(words), tt.expected)
			}
		})
	}
}

// TestAddStaticWords 测试添加静态敏感词
func TestAddStaticWords(t *testing.T) {
	config := DefaultSensitiveWordsConfig()

	// 添加英语敏感词
	config.AddStaticWords("en", []string{"bad", "worse"})

	if len(config.StaticWords["en"]) != 2 {
		t.Errorf("StaticWords[en] 长度 = %d, 期望 2", len(config.StaticWords["en"]))
	}

	// 再次添加
	config.AddStaticWords("en", []string{"terrible"})

	if len(config.StaticWords["en"]) != 3 {
		t.Errorf("StaticWords[en] 长度 = %d, 期望 3", len(config.StaticWords["en"]))
	}

	// 添加中文敏感词
	config.AddStaticWords("zh", []string{"敏感"})

	if len(config.StaticWords["zh"]) != 1 {
		t.Errorf("StaticWords[zh] 长度 = %d, 期望 1", len(config.StaticWords["zh"]))
	}
}

// TestAddDynamicWords 测试添加动态敏感词
func TestAddDynamicWords(t *testing.T) {
	config := DefaultSensitiveWordsConfig()

	// 添加英语敏感词
	config.AddDynamicWords("en", []string{"spam"})

	if len(config.DynamicWords["en"]) != 1 {
		t.Errorf("DynamicWords[en] 长度 = %d, 期望 1", len(config.DynamicWords["en"]))
	}

	// 再次添加
	config.AddDynamicWords("en", []string{"scam", "fraud"})

	if len(config.DynamicWords["en"]) != 3 {
		t.Errorf("DynamicWords[en] 长度 = %d, 期望 3", len(config.DynamicWords["en"]))
	}
}

// TestAddWordsToNilMaps 测试向nil map添加敏感词
func TestAddWordsToNilMaps(t *testing.T) {
	config := &SensitiveWordsConfig{
		StaticWords:  nil,
		DynamicWords: nil,
	}

	// 应该自动初始化map
	config.AddStaticWords("en", []string{"test"})
	if config.StaticWords == nil {
		t.Error("AddStaticWords 应该初始化 StaticWords")
	}
	if len(config.StaticWords["en"]) != 1 {
		t.Errorf("StaticWords[en] 长度 = %d, 期望 1", len(config.StaticWords["en"]))
	}

	config.AddDynamicWords("en", []string{"test"})
	if config.DynamicWords == nil {
		t.Error("AddDynamicWords 应该初始化 DynamicWords")
	}
	if len(config.DynamicWords["en"]) != 1 {
		t.Errorf("DynamicWords[en] 长度 = %d, 期望 1", len(config.DynamicWords["en"]))
	}
}

// TestSensitiveWordsConfigMetadata 测试敏感词配置元数据
func TestSensitiveWordsConfigMetadata(t *testing.T) {
	config := &SensitiveWordsConfig{
		StaticWords:  make(map[string][]string),
		DynamicWords: make(map[string][]string),
		LastUpdated:  "2026-03-06",
		Version:      "1.0.0",
		Platform:     "TEMU",
	}

	if config.LastUpdated != "2026-03-06" {
		t.Errorf("LastUpdated = %q, 期望 '2026-03-06'", config.LastUpdated)
	}

	if config.Version != "1.0.0" {
		t.Errorf("Version = %q, 期望 '1.0.0'", config.Version)
	}

	if config.Platform != "TEMU" {
		t.Errorf("Platform = %q, 期望 'TEMU'", config.Platform)
	}
}

// BenchmarkCalculateTotalCost 性能基准测试
func BenchmarkCalculateTotalCost(b *testing.B) {
	config := &CostCalculationConfig{
		FixedCostAmount:    5.0,
		FixedCostPercent:   10.0,
		ShippingCost:       8.0,
		ProcessingFee:      3.0,
		PlatformCommission: 5.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.CalculateTotalCost(100.0)
	}
}

// BenchmarkGetWordsForLanguage 性能基准测试
func BenchmarkGetWordsForLanguage(b *testing.B) {
	config := &SensitiveWordsConfig{
		StaticWords: map[string][]string{
			"en": {"bad", "worse", "terrible", "awful", "horrible"},
		},
		DynamicWords: map[string][]string{
			"en": {"spam", "scam", "fraud", "fake", "phishing"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.GetWordsForLanguage("en")
	}
}
