// Package handlers 提供TEMU平台相关的处理器
package handlers

import (
	"task-processor/platforms/temu/types"
	"task-processor/platforms/temu/utils"
	"testing"
)

// TestImageDimensionFix 测试图片尺寸修复功能（直接测试验证器）
func TestImageDimensionFix(t *testing.T) {
	// 直接测试我们的尺寸验证工具
	validator := utils.NewImageDimensionValidator()

	tests := []struct {
		name        string
		product     *types.Product
		expectError bool
		description string
	}{
		{
			name: "801x800问题图片应该被捕获",
			product: &types.Product{
				GoodsBasic: types.GoodsBasic{
					IsClothes: false,
					GoodsGallery: types.GoodsGallery{
						DetailImage: []types.ImageInfo{
							{Width: 801, Height: 800, URL: "test1.jpg"}, // 问题图片
						},
					},
				},
				SkcList: []types.Skc{},
			},
			expectError: true,
			description: "801x800非1:1比例图片应该被验证器捕获",
		},
		{
			name: "正确的1:1比例图片",
			product: &types.Product{
				GoodsBasic: types.GoodsBasic{
					IsClothes: false,
					GoodsGallery: types.GoodsGallery{
						DetailImage: []types.ImageInfo{
							{Width: 800, Height: 800, URL: "test1.jpg"},
							{Width: 1000, Height: 1000, URL: "test2.jpg"},
						},
					},
				},
				SkcList: []types.Skc{},
			},
			expectError: false,
			description: "所有图片都是正确的1:1比例",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateProductImages(tt.product)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望出现错误（%s），但没有错误", tt.description)
				} else {
					t.Logf("✅ 成功捕获预期错误: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误（%s），但出现了: %v", tt.description, err)
				} else {
					t.Logf("✅ 验证通过: %s", tt.description)
				}
			}
		})
	}
}

// TestImagePaddingProcessor 测试图片填充处理器的1:1比例强制功能
func TestImagePaddingProcessor(t *testing.T) {
	processor := NewImagePaddingProcessor()

	tests := []struct {
		name           string
		width          int
		height         int
		targetRatio    float64
		minWidth       int
		minHeight      int
		expectedWidth  int
		expectedHeight int
		description    string
	}{
		{
			name:           "801x800应该被填充为正方形",
			width:          801,
			height:         800,
			targetRatio:    1.0,
			minWidth:       800,
			minHeight:      800,
			expectedWidth:  801,
			expectedHeight: 801,
			description:    "宽度较大，应该以宽度为准填充高度",
		},
		{
			name:           "800x801应该被填充为正方形",
			width:          800,
			height:         801,
			targetRatio:    1.0,
			minWidth:       800,
			minHeight:      800,
			expectedWidth:  801,
			expectedHeight: 801,
			description:    "高度较大，应该以高度为准填充宽度",
		},
		{
			name:           "已经是1:1的图片不需要填充",
			width:          1000,
			height:         1000,
			targetRatio:    1.0,
			minWidth:       800,
			minHeight:      800,
			expectedWidth:  1000,
			expectedHeight: 1000,
			description:    "完美的1:1比例，无需填充",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newWidth, newHeight, needsPadding := processor.CalculatePaddingDimensions(
				tt.width, tt.height, tt.targetRatio, tt.minWidth, tt.minHeight)

			if newWidth != tt.expectedWidth || newHeight != tt.expectedHeight {
				t.Errorf("尺寸计算错误: 期望 %dx%d, 实际 %dx%d (%s)",
					tt.expectedWidth, tt.expectedHeight, newWidth, newHeight, tt.description)
			}

			// 验证1:1比例
			if tt.targetRatio == 1.0 && newWidth != newHeight {
				t.Errorf("1:1比例验证失败: %dx%d 不是正方形", newWidth, newHeight)
			}

			t.Logf("✅ %s: %dx%d -> %dx%d (需要填充: %v)",
				tt.description, tt.width, tt.height, newWidth, newHeight, needsPadding)
		})
	}
}
