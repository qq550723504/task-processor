// package activity 提供SHEIN平台调度器相关服务
package activity

import (
	"testing"

	"task-processor/internal/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

func TestActivityStockValidation(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	service := &activityRegistrationServiceImpl{
		logger: logger,
	}

	tests := []struct {
		name        string
		totalStock  int
		stockRatio  float64
		expectSkip  bool
		description string
	}{
		{
			name:        "正常库存",
			totalStock:  100,
			stockRatio:  0.5,
			expectSkip:  false,
			description: "库存100，比例50%，应该得到50的活动库存",
		},
		{
			name:        "零库存应该跳过",
			totalStock:  0,
			stockRatio:  0.5,
			expectSkip:  true,
			description: "库存为0时应该跳过该产品",
		},
		{
			name:        "负库存应该跳过",
			totalStock:  -1,
			stockRatio:  0.5,
			expectSkip:  true,
			description: "负库存时应该跳过该产品",
		},
		{
			name:        "小库存低比例",
			totalStock:  1,
			stockRatio:  0.1,
			expectSkip:  false,
			description: "库存1，比例10%，应该得到1的活动库存（最小值保护）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟产品数据
			product := marketing.SkcInfo{
				Skc:   "TEST_SKC_" + tt.name,
				Stock: tt.totalStock,
			}

			// 计算活动库存
			actStock := service.calculateActivityStock(product.Stock, tt.stockRatio)

			// 验证是否应该跳过
			shouldSkip := actStock <= 0 || product.Stock <= 0

			if shouldSkip != tt.expectSkip {
				t.Errorf("期望跳过: %v, 实际跳过: %v (活动库存: %d, 原库存: %d)",
					tt.expectSkip, shouldSkip, actStock, product.Stock)
			}

			// 如果不跳过，验证活动库存是正整数
			if !shouldSkip && actStock <= 0 {
				t.Errorf("活动库存应该是正整数，实际值: %d", actStock)
			}

			t.Logf("%s - 原库存: %d, 活动库存: %d, 跳过: %v",
				tt.description, product.Stock, actStock, shouldSkip)
		})
	}
}

func TestActivityConfigValidation(t *testing.T) {
	// 测试ActivityConfig的字段验证
	tests := []struct {
		name   string
		config marketing.ActivityConfig
		valid  bool
	}{
		{
			name: "有效配置",
			config: marketing.ActivityConfig{
				Skc:              "TEST_SKC_001",
				ActStock:         10,
				DropRate:         15,
				ReservedActStock: 20,
			},
			valid: true,
		},
		{
			name: "活动库存为0",
			config: marketing.ActivityConfig{
				Skc:              "TEST_SKC_002",
				ActStock:         0,
				DropRate:         15,
				ReservedActStock: 20,
			},
			valid: false,
		},
		{
			name: "预留库存为0",
			config: marketing.ActivityConfig{
				Skc:              "TEST_SKC_003",
				ActStock:         10,
				DropRate:         15,
				ReservedActStock: 0,
			},
			valid: false,
		},
		{
			name: "降幅为0",
			config: marketing.ActivityConfig{
				Skc:              "TEST_SKC_004",
				ActStock:         10,
				DropRate:         0,
				ReservedActStock: 20,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证所有字段都是正整数
			isValid := tt.config.ActStock > 0 &&
				tt.config.DropRate > 0 &&
				tt.config.ReservedActStock > 0

			if isValid != tt.valid {
				t.Errorf("期望有效性: %v, 实际有效性: %v (ActStock: %d, DropRate: %d, ReservedActStock: %d)",
					tt.valid, isValid, tt.config.ActStock, tt.config.DropRate, tt.config.ReservedActStock)
			}
		})
	}
}
