// Package rules 提供TEMU平台的规则验证功能测试
package rules

import (
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestCheckImageCountRuleDetailed 测试图片数量规则检查
func TestCheckImageCountRuleDetailed(t *testing.T) {
	// 创建测试用的规则验证器
	logger := logrus.NewEntry(logrus.New())
	validator := NewRuleValidator(logger)

	tests := []struct {
		name           string
		product        *model.Product
		expectedPassed bool
		expectedReason string
	}{
		{
			name: "图片数量足够_应该通过",
			product: &model.Product{
				Asin:   "B123456789",
				Images: []string{"img1.jpg", "img2.jpg", "img3.jpg", "img4.jpg"},
			},
			expectedPassed: true,
			expectedReason: "",
		},
		{
			name: "图片数量刚好3张_应该通过",
			product: &model.Product{
				Asin:   "B123456789",
				Images: []string{"img1.jpg", "img2.jpg", "img3.jpg"},
			},
			expectedPassed: true,
			expectedReason: "",
		},
		{
			name: "图片数量不足2张_应该失败",
			product: &model.Product{
				Asin:   "B123456789",
				Images: []string{"img1.jpg", "img2.jpg"},
			},
			expectedPassed: false,
			expectedReason: "Amazon原始数据图片不足，当前2张，至少需要3张",
		},
		{
			name: "图片数量不足1张_应该失败",
			product: &model.Product{
				Asin:   "B123456789",
				Images: []string{"img1.jpg"},
			},
			expectedPassed: false,
			expectedReason: "Amazon原始数据图片不足，当前1张，至少需要3张",
		},
		{
			name: "没有图片_应该失败",
			product: &model.Product{
				Asin:   "B123456789",
				Images: []string{},
			},
			expectedPassed: false,
			expectedReason: "Amazon原始数据图片不足，当前0张，至少需要3张",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.checkImageCountRuleDetailed(tt.product)

			assert.Equal(t, tt.expectedPassed, result.Passed, "验证结果不符合预期")

			if !tt.expectedPassed {
				assert.Equal(t, tt.expectedReason, result.FailureReason, "失败原因不符合预期")
				assert.Equal(t, float64(len(tt.product.Images)), result.ProductValue, "产品值不符合预期")
				assert.Equal(t, float64(3), result.RuleValue, "规则值不符合预期")
			}
		})
	}
}

// TestCheckSingleRuleDetailedTemu_WithImageValidation 测试完整规则检查包含图片验证
func TestCheckSingleRuleDetailedTemu_WithImageValidation(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	validator := NewRuleValidator(logger)

	// 测试图片不足的情况应该直接失败，不进行其他规则检查
	product := &model.Product{
		Asin:         "B123456789",
		Images:       []string{"img1.jpg"}, // 只有1张图片，不足3张
		FinalPrice:   10.0,
		Rating:       4.5,
		ReviewsCount: 100,
	}

	rule := &api.FilterRuleRespDTO{
		PriceMin:       &[]float64{5.0}[0],
		PriceMax:       &[]float64{20.0}[0],
		RatingMin:      &[]float64{4.0}[0],
		ReviewCountMin: &[]int{50}[0],
	}

	result := validator.CheckSingleRuleDetailedTemu(product, rule, nil)

	// 应该因为图片数量不足而失败
	assert.False(t, result.Passed, "应该因为图片数量不足而失败")
	assert.Contains(t, result.FailureReason, "Amazon原始数据图片不足", "失败原因应该包含图片不足的信息")
}
