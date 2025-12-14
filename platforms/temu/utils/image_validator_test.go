package utils

import (
	"strings"
	"task-processor/platforms/temu/types"
	"testing"
)

func TestImageDimensionValidator_ValidateProductImages(t *testing.T) {
	validator := NewImageDimensionValidator()

	tests := []struct {
		name        string
		product     *types.Product
		expectError bool
		errorMsg    string
	}{
		{
			name: "正确的1:1比例图片",
			product: &types.Product{
				GoodsBasic: types.GoodsBasic{
					IsClothes: false,
					GoodsGallery: types.GoodsGallery{
						DetailImage: []types.ImageInfo{
							{Width: 800, Height: 800},
							{Width: 1000, Height: 1000},
						},
					},
				},
				SkcList: []types.Skc{
					{
						SkuList: []types.Sku{
							{
								CarouselGallery: []types.ImageInfo{
									{Width: 1200, Height: 1200},
								},
								DimensionGallery: []types.ImageInfo{
									{Width: 900, Height: 900},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "错误的非1:1比例图片",
			product: &types.Product{
				GoodsBasic: types.GoodsBasic{
					IsClothes: false,
					GoodsGallery: types.GoodsGallery{
						DetailImage: []types.ImageInfo{
							{Width: 801, Height: 800}, // 问题图片
						},
					},
				},
				SkcList: []types.Skc{},
			},
			expectError: true,
			errorMsg:    "非服装类图片必须为1:1比例",
		},
		{
			name: "正确的服装类3:4比例图片",
			product: &types.Product{
				GoodsBasic: types.GoodsBasic{
					IsClothes: true,
					GoodsGallery: types.GoodsGallery{
						DetailImage: []types.ImageInfo{
							{Width: 1340, Height: 1785}, // 3:4比例
						},
					},
				},
				SkcList: []types.Skc{},
			},
			expectError: false,
		},
		{
			name: "尺寸不足的图片",
			product: &types.Product{
				GoodsBasic: types.GoodsBasic{
					IsClothes: false,
					GoodsGallery: types.GoodsGallery{
						DetailImage: []types.ImageInfo{
							{Width: 500, Height: 500}, // 尺寸不足
						},
					},
				},
				SkcList: []types.Skc{},
			},
			expectError: true,
			errorMsg:    "尺寸不足",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateProductImages(tt.product)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望出现错误，但没有错误")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("错误信息不匹配，期望包含: %s, 实际: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("不期望出现错误，但出现了: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				strings.Contains(s, substr))))
}
