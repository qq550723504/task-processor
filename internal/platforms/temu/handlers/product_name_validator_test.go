package handlers

import (
	"testing"

	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/stretchr/testify/assert"
)

func TestProductNameValidator_validateAndCleanProductName(t *testing.T) {
	validator := NewProductNameValidator()

	tests := []struct {
		name             string
		input            string
		expectedOutput   string
		expectViolations bool
	}{
		{
			name:             "正常的产品名称",
			input:            "Computer Gaming Chair Office Ergonomic PC Chair",
			expectedOutput:   "Computer Gaming Chair Office Ergonomic PC Chair",
			expectViolations: false,
		},
		{
			name:             "包含装饰字符",
			input:            "Gaming Chair~ with *Special* Features!",
			expectedOutput:   "Gaming Chair with Special Features",
			expectViolations: true,
		},
		{
			name:             "包含高ASCII字符",
			input:            "Gaming Chair® with Copyright© and Trademark™",
			expectedOutput:   "Gaming Chair (R) with Copyright (C) and Trademark (TM)",
			expectViolations: true,
		},
		{
			name:             "包含不支持的符号",
			input:            "Chair {with} <brackets> |and| pipes",
			expectedOutput:   "Chair with brackets and pipes",
			expectViolations: true,
		},
		{
			name:             "多余的空格",
			input:            "  Gaming   Chair    with   spaces  ",
			expectedOutput:   "Gaming Chair with spaces",
			expectViolations: false,
		},
		{
			name:             "混合问题",
			input:            "Gaming® Chair~ with *Special* {Features}!  ",
			expectedOutput:   "Gaming (R) Chair with Special Features",
			expectViolations: true,
		},
		{
			name:             "括号前缺少空格",
			input:            "Gaming Chair(Ergonomic Design)",
			expectedOutput:   "Gaming Chair (Ergonomic Design)",
			expectViolations: false,
		},
		{
			name:             "多个括号前缺少空格",
			input:            "Chair(Red)with Cushion(Soft)",
			expectedOutput:   "Chair (Red) with Cushion (Soft)",
			expectViolations: false,
		},
		{
			name:             "括号前已有空格",
			input:            "Gaming Chair (Ergonomic Design)",
			expectedOutput:   "Gaming Chair (Ergonomic Design)",
			expectViolations: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, violations := validator.validateAndCleanProductName(tt.input)

			assert.Equal(t, tt.expectedOutput, cleaned)

			if tt.expectViolations {
				assert.NotEmpty(t, violations, "应该检测到违规")
			} else {
				assert.Empty(t, violations, "不应该有违规")
			}
		})
	}
}

func TestProductNameValidator_Handle(t *testing.T) {
	validator := NewProductNameValidator()

	tests := []struct {
		name        string
		productName string
		expectError bool
	}{
		{
			name:        "正常产品名称",
			productName: "Computer Gaming Chair Office Ergonomic PC Chair",
			expectError: false,
		},
		{
			name:        "需要清理的产品名称",
			productName: "Gaming Chair® with *Special* Features!",
			expectError: false,
		},
		{
			name:        "空产品名称",
			productName: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						GoodsName: tt.productName,
					},
				},
			}

			err := validator.Handle(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// 验证产品名称已被清理
				assert.NotEmpty(t, ctx.TemuProduct.GoodsBasic.GoodsName)
				// 验证长度不超过500字符
				assert.LessOrEqual(t, len(ctx.TemuProduct.GoodsBasic.GoodsName), 500)
			}
		})
	}
}

func TestProductNameValidator_LengthLimit(t *testing.T) {
	validator := NewProductNameValidator()

	// 创建一个超过500字符的产品名称
	longName := ""
	for i := 0; i < 600; i++ {
		longName += "a"
	}

	ctx := &pipeline.TaskContext{
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				GoodsName: longName,
			},
		},
	}

	err := validator.Handle(ctx)
	assert.NoError(t, err)

	// 验证名称被截断到500字符
	assert.Equal(t, 500, len(ctx.TemuProduct.GoodsBasic.GoodsName))
}
