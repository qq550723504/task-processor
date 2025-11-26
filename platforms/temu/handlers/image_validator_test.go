package handlers

import (
	"testing"

	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/stretchr/testify/assert"
)

func TestImageValidator_validateSingleImage(t *testing.T) {
	validator := NewImageValidator()

	// 通用图片要求（1:1）
	generalRequirement := ImageRequirement{
		MaxSizeMB:     3.0,
		MinWidth:      800,
		MinHeight:     800,
		AspectRatio:   1.0,
		MinImageCount: 1,
		MaxImageCount: 10,
	}

	tests := []struct {
		name             string
		imageURL         string
		expectValid      bool
		expectViolations int
	}{
		{
			name:             "有效的JPEG图片",
			imageURL:         "https://example.com/image.jpg",
			expectValid:      true,
			expectViolations: 0,
		},
		{
			name:             "有效的PNG图片",
			imageURL:         "https://example.com/image.png",
			expectValid:      true,
			expectViolations: 0,
		},
		{
			name:             "不支持的格式",
			imageURL:         "https://example.com/image.gif",
			expectValid:      false,
			expectViolations: 1,
		},
		{
			name:             "空URL",
			imageURL:         "",
			expectValid:      false,
			expectViolations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateSingleImage(tt.imageURL, "测试图片", generalRequirement)

			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectViolations, len(result.Violations))
			assert.Equal(t, tt.imageURL, result.URL)
		})
	}
}

func TestImageValidator_getImageFormat(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		imageURL string
		expected string
	}{
		{"https://example.com/image.jpg", "JPEG"},
		{"https://example.com/image.jpeg", "JPEG"},
		{"https://example.com/image.png", "PNG"},
		{"https://example.com/image.JPG", "JPEG"},
		{"https://example.com/image.PNG", "PNG"},
		{"https://example.com/image.gif", ".gif"},
		{"https://example.com/image", ""},
	}

	for _, tt := range tests {
		t.Run(tt.imageURL, func(t *testing.T) {
			result := validator.getImageFormat(tt.imageURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_isValidFormat(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		format   string
		expected bool
	}{
		{"JPEG", true},
		{"JPG", true},
		{"PNG", true},
		{"jpeg", true},
		{"jpg", true},
		{"png", true},
		{"GIF", false},
		{"BMP", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := validator.isValidFormat(tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_Handle(t *testing.T) {
	validator := NewImageValidator()

	// 创建测试上下文 - 非服装类产品
	ctx := &pipeline.TaskContext{
		Data: make(map[string]interface{}), // 初始化Data map
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				IsClothes: false,
				GoodsGallery: types.GoodsGallery{
					DetailImage: []types.ImageInfo{
						{URL: "https://example.com/image1.jpg"},
						{URL: "https://example.com/image2.png"},
					},
				},
			},
			SkcList: []types.Skc{
				{
					SkuList: []types.Sku{
						{
							CarouselGallery: []types.ImageInfo{
								{URL: "https://example.com/carousel1.jpg"},
							},
							DimensionGallery: []types.ImageInfo{
								{URL: "https://example.com/dimension1.jpg"},
							},
						},
					},
				},
			},
		},
	}

	err := validator.Handle(ctx)
	assert.NoError(t, err)

	// 验证图片信息被更新
	assert.NotEmpty(t, ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage)
	assert.NotEmpty(t, ctx.TemuProduct.SkcList[0].SkuList[0].CarouselGallery)
	assert.NotEmpty(t, ctx.TemuProduct.SkcList[0].SkuList[0].DimensionGallery)
}

func TestImageValidator_GetImageRequirement(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name              string
		isClothes         bool
		expectedMinWidth  int
		expectedMinHeight int
		expectedRatio     float64
	}{
		{
			name:              "服装类产品",
			isClothes:         true,
			expectedMinWidth:  1340,
			expectedMinHeight: 1785,
			expectedRatio:     0.75, // 3:4
		},
		{
			name:              "非服装类产品",
			isClothes:         false,
			expectedMinWidth:  800,
			expectedMinHeight: 800,
			expectedRatio:     1.0, // 1:1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						IsClothes: tt.isClothes,
					},
				},
			}

			requirement := validator.getImageRequirement(ctx)

			assert.Equal(t, tt.expectedMinWidth, requirement.MinWidth)
			assert.Equal(t, tt.expectedMinHeight, requirement.MinHeight)
			assert.Equal(t, tt.expectedRatio, requirement.AspectRatio)
			assert.Equal(t, 3.0, requirement.MaxSizeMB)
			assert.Equal(t, 1, requirement.MinImageCount)
			assert.Equal(t, 10, requirement.MaxImageCount)
		})
	}
}

func TestImageValidator_GetImageValidationSummary(t *testing.T) {
	validator := NewImageValidator()

	ctx := &pipeline.TaskContext{
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				GoodsGallery: types.GoodsGallery{
					DetailImage: []types.ImageInfo{
						{URL: "https://example.com/image1.jpg"},
						{URL: "https://example.com/image2.png"},
					},
				},
			},
			SkcList: []types.Skc{
				{
					SkuList: []types.Sku{
						{
							CarouselGallery: []types.ImageInfo{
								{URL: "https://example.com/carousel1.jpg"},
							},
							DimensionGallery: []types.ImageInfo{
								{URL: "https://example.com/dimension1.jpg"},
							},
						},
					},
				},
			},
		},
	}

	summary := validator.GetImageValidationSummary(ctx)

	assert.Equal(t, 2, summary["main_images"])
	assert.Equal(t, 2, summary["sku_images"])
	assert.Equal(t, 4, summary["total_images"])
	assert.Equal(t, true, summary["requires_upload"])
}
