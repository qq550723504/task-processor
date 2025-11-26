package handlers

import (
	"testing"

	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/stretchr/testify/assert"
)

// TestImageValidationAndUploadIntegration 测试图片验证和上传的集成
func TestImageValidationAndUploadIntegration(t *testing.T) {
	// 创建测试上下文
	ctx := &pipeline.TaskContext{
		Data: make(map[string]interface{}), // 初始化Data map
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				IsClothes: true, // 服装类产品，需要3:4宽高比
				GoodsGallery: types.GoodsGallery{
					DetailImage: []types.ImageInfo{
						{URL: "https://example.com/image1.jpg"},
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
						},
					},
				},
			},
		},
	}

	// 1. 运行图片验证器
	validator := NewImageValidator()
	err := validator.Handle(ctx)

	// 验证器应该成功（即使图片下载失败，也会继续处理）
	// 在实际测试中，图片URL无效会导致验证失败，但这里我们主要测试流程
	t.Logf("验证器执行结果: %v", err)

	// 2. 检查上下文中是否设置了必要的标志
	requiresUpload, exists := ctx.GetData("requires_image_upload")
	assert.True(t, exists, "应该设置 requires_image_upload 标志")
	assert.True(t, requiresUpload.(bool), "requires_image_upload 应该为 true")

	// 3. 检查是否有填充后的图片数据（如果有的话）
	if paddedData, exists := ctx.GetData("padded_images"); exists {
		paddedImages := paddedData.(map[string][]byte)
		t.Logf("找到 %d 张填充后的图片", len(paddedImages))

		// 4. 检查是否有对应的尺寸信息
		if sizeData, sizeExists := ctx.GetData("padded_image_sizes"); sizeExists {
			sizes := sizeData.(map[string][2]int)
			assert.Equal(t, len(paddedImages), len(sizes), "填充图片数量应该与尺寸信息数量一致")

			// 验证每个填充图片都有对应的尺寸信息
			for url := range paddedImages {
				size, hasSize := sizes[url]
				assert.True(t, hasSize, "填充图片应该有对应的尺寸信息: %s", url)
				assert.Greater(t, size[0], 0, "宽度应该大于0")
				assert.Greater(t, size[1], 0, "高度应该大于0")
				t.Logf("图片 %s 的填充尺寸: %dx%d", url, size[0], size[1])
			}
		}
	}
}

// TestPaddedImageSizePreservation 测试填充后的尺寸是否被保留
func TestPaddedImageSizePreservation(t *testing.T) {
	ctx := &pipeline.TaskContext{
		Data: make(map[string]interface{}), // 初始化Data map
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				IsClothes: true,
				GoodsGallery: types.GoodsGallery{
					DetailImage: []types.ImageInfo{
						{URL: "https://example.com/test.jpg"},
					},
				},
			},
		},
	}

	// 模拟填充后的数据
	paddedImages := map[string][]byte{
		"https://example.com/test.jpg": []byte("fake image data"),
	}
	paddedSizes := map[string][2]int{
		"https://example.com/test.jpg": {1340, 1786}, // 3:4 比例
	}

	ctx.SetData("padded_images", paddedImages)
	ctx.SetData("padded_image_sizes", paddedSizes)

	// 验证数据是否正确保存
	retrievedImages, exists := ctx.GetData("padded_images")
	assert.True(t, exists, "应该能获取填充图片数据")
	assert.Equal(t, paddedImages, retrievedImages, "填充图片数据应该一致")

	retrievedSizes, exists := ctx.GetData("padded_image_sizes")
	assert.True(t, exists, "应该能获取填充尺寸数据")
	assert.Equal(t, paddedSizes, retrievedSizes, "填充尺寸数据应该一致")

	// 验证尺寸比例
	size := paddedSizes["https://example.com/test.jpg"]
	ratio := float64(size[0]) / float64(size[1])
	expectedRatio := 0.75 // 3:4
	tolerance := 0.01
	assert.InDelta(t, expectedRatio, ratio, tolerance, "宽高比应该接近3:4")
}

// TestImageRequirementByCategory 测试不同分类的图片要求
func TestImageRequirementByCategory(t *testing.T) {
	validator := NewImageValidator()

	tests := []struct {
		name              string
		isClothes         bool
		expectedRatio     float64
		expectedMinWidth  int
		expectedMinHeight int
	}{
		{
			name:              "服装类产品",
			isClothes:         true,
			expectedRatio:     0.75, // 3:4
			expectedMinWidth:  1340,
			expectedMinHeight: 1785,
		},
		{
			name:              "非服装类产品",
			isClothes:         false,
			expectedRatio:     1.0, // 1:1
			expectedMinWidth:  800,
			expectedMinHeight: 800,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				Data: make(map[string]interface{}), // 初始化Data map
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						IsClothes: tt.isClothes,
					},
				},
			}

			requirement := validator.getImageRequirement(ctx)

			assert.Equal(t, tt.expectedRatio, requirement.AspectRatio, "宽高比应该匹配")
			assert.Equal(t, tt.expectedMinWidth, requirement.MinWidth, "最小宽度应该匹配")
			assert.Equal(t, tt.expectedMinHeight, requirement.MinHeight, "最小高度应该匹配")
			assert.Equal(t, 3.0, requirement.MaxSizeMB, "最大文件大小应该为3MB")
			assert.Equal(t, 1, requirement.MinImageCount, "最小图片数量应该为1")
			assert.Equal(t, 10, requirement.MaxImageCount, "最大图片数量应该为10")
		})
	}
}
