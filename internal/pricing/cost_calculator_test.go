package pricing

import (
	"testing"

	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// TestNewCostCalculator 测试创建成本计算器
func TestNewCostCalculator(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	tests := []struct {
		name            string
		enableDetailLog bool
	}{
		{
			name:            "创建简洁日志模式的计算器",
			enableDetailLog: false,
		},
		{
			name:            "创建详细日志模式的计算器",
			enableDetailLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 使用nil作为managementClient，因为默认配置不需要它
			calc := NewCostCalculator(nil, logger, tt.enableDetailLog)
			if calc == nil {
				t.Error("NewCostCalculator 返回 nil")
			}
			if calc.enableDetailLog != tt.enableDetailLog {
				t.Errorf("enableDetailLog = %v, 期望 %v", calc.enableDetailLog, tt.enableDetailLog)
			}
		})
	}
}

// TestCalculateProductCost 测试产品成本计算
func TestCalculateProductCost(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	// 使用nil作为managementClient，测试默认配置
	calc := NewCostCalculator(nil, logger, false)

	tests := []struct {
		name          string
		baseCostPrice float64
		storeID       int64
		productID     string
		sku           string
		expected      float64
	}{
		{
			name:          "正常成本计算",
			baseCostPrice: 100.0,
			storeID:       1,
			productID:     "P001",
			sku:           "SKU001",
			expected:      100.0, // 使用默认配置，无额外成本
		},
		{
			name:          "零成本价格",
			baseCostPrice: 0,
			storeID:       1,
			productID:     "P002",
			sku:           "SKU002",
			expected:      0,
		},
		{
			name:          "负数成本价格",
			baseCostPrice: -10.0,
			storeID:       1,
			productID:     "P003",
			sku:           "SKU003",
			expected:      0, // 负数应返回0
		},
		{
			name:          "小数成本价格",
			baseCostPrice: 99.99,
			storeID:       1,
			productID:     "P004",
			sku:           "SKU004",
			expected:      99.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateProductCost(tt.baseCostPrice, tt.storeID, tt.productID, tt.sku)
			if result != tt.expected {
				t.Errorf("CalculateProductCost() = %v, 期望 %v", result, tt.expected)
			}
		})
	}
}

// TestCalculateAmazonProductCost 测试Amazon产品成本计算
func TestCalculateAmazonProductCost(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	calc := NewCostCalculator(nil, logger, false)

	tests := []struct {
		name          string
		amazonProduct *model.Product
		priceType     string
		storeID       int64
		expected      float64
	}{
		{
			name: "正常Amazon产品成本计算-special价格",
			amazonProduct: &model.Product{
				Asin:       "B001",
				FinalPrice: 50.0,
			},
			priceType: "special",
			storeID:   1,
			expected:  50.0,
		},
		{
			name: "正常Amazon产品成本计算-original价格",
			amazonProduct: &model.Product{
				Asin:         "B002",
				InitialPrice: 75.0,
			},
			priceType: "original",
			storeID:   1,
			expected:  75.0,
		},
		{
			name: "零价格Amazon产品",
			amazonProduct: &model.Product{
				Asin:       "B003",
				FinalPrice: 0,
			},
			priceType: "special",
			storeID:   1,
			expected:  0,
		},
		{
			name: "默认价格类型",
			amazonProduct: &model.Product{
				Asin:       "B004",
				FinalPrice: 99.99,
			},
			priceType: "unknown", // 未知类型，应使用默认的FinalPrice
			storeID:   1,
			expected:  99.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateAmazonProductCost(tt.amazonProduct, tt.priceType, tt.storeID)
			if result != tt.expected {
				t.Errorf("CalculateAmazonProductCost() = %v, 期望 %v", result, tt.expected)
			}
		})
	}
}

// TestGetCostConfig 测试获取成本配置（使用默认配置）
func TestGetCostConfig(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	calc := NewCostCalculator(nil, logger, false)

	// 测试默认配置
	config := calc.getCostConfig(1)
	if config == nil {
		t.Error("getCostConfig 返回 nil")
	}

	// 验证默认配置值
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

// TestDetailedLogging 测试详细日志功能
func TestDetailedLogging(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	tests := []struct {
		name            string
		enableDetailLog bool
	}{
		{
			name:            "启用详细日志",
			enableDetailLog: true,
		},
		{
			name:            "禁用详细日志",
			enableDetailLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewCostCalculator(nil, logger, tt.enableDetailLog)

			// 调用计算方法，验证不会崩溃
			result := calc.CalculateProductCost(100.0, 1, "P001", "SKU001")
			if result != 100.0 {
				t.Errorf("CalculateProductCost() = %v, 期望 100.0", result)
			}

			// 测试Amazon产品计算
			product := &model.Product{
				Asin:       "B001",
				FinalPrice: 50.0,
			}
			result2 := calc.CalculateAmazonProductCost(product, "special", 1)
			if result2 != 50.0 {
				t.Errorf("CalculateAmazonProductCost() = %v, 期望 50.0", result2)
			}
		})
	}
}

// BenchmarkCalculateProductCost 性能基准测试
func BenchmarkCalculateProductCost(b *testing.B) {
	logger := logrus.NewEntry(logrus.New())
	calc := NewCostCalculator(nil, logger, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateProductCost(100.0, 1, "P001", "SKU001")
	}
}

// BenchmarkCalculateAmazonProductCost 性能基准测试
func BenchmarkCalculateAmazonProductCost(b *testing.B) {
	logger := logrus.NewEntry(logrus.New())
	calc := NewCostCalculator(nil, logger, false)

	product := &model.Product{
		Asin:       "B001",
		FinalPrice: 50.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateAmazonProductCost(product, "special", 1)
	}
}
