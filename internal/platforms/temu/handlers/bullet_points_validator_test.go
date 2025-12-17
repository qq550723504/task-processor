package handlers

import (
	"strings"
	"testing"

	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/stretchr/testify/assert"
)

func TestBulletPointsValidator_validateAndOptimizeBulletPoints(t *testing.T) {
	validator := NewBulletPointsValidator()

	tests := []struct {
		name              string
		inputPoints       []string
		expectValid       bool
		expectViolations  int
		expectSuggestions int
		maxExpectedLength int
	}{
		{
			name: "正常的要点",
			inputPoints: []string{
				"Ergonomic design for comfortable seating",
				"High-quality materials for durability",
				"Adjustable height and tilt features",
			},
			expectValid:       true,
			expectViolations:  0,
			expectSuggestions: 0,
			maxExpectedLength: 700,
		},
		{
			name: "要点数量超限",
			inputPoints: []string{
				"Point 1", "Point 2", "Point 3", "Point 4",
				"Point 5", "Point 6", "Point 7", "Point 8",
			},
			expectValid:       false,
			expectViolations:  1,
			expectSuggestions: 0,
			maxExpectedLength: 700,
		},
		{
			name: "包含空要点",
			inputPoints: []string{
				"Valid point",
				"",
				"Another valid point",
			},
			expectValid:       false,
			expectViolations:  1,
			expectSuggestions: 0,
			maxExpectedLength: 700,
		},
		{
			name: "要点需要格式优化",
			inputPoints: []string{
				"ergonomic design for comfort.",
				"  high quality materials  ",
				"adjustable features",
			},
			expectValid:       true,
			expectViolations:  0,
			expectSuggestions: 2, // 格式优化建议
			maxExpectedLength: 700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateAndOptimizeBulletPoints(tt.inputPoints)

			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectViolations, len(result.Violations))
			assert.LessOrEqual(t, len(result.ValidatedPoints), 6) // 最多6个要点
			assert.LessOrEqual(t, result.TotalLength, tt.maxExpectedLength)

			// 验证所有要点都不为空
			for _, point := range result.ValidatedPoints {
				assert.NotEmpty(t, point)
			}
		})
	}
}

func TestBulletPointsValidator_removeUnsupportedChars(t *testing.T) {
	validator := NewBulletPointsValidator()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Ergonomic design with special features",
			expected: "Ergonomic design with special features",
		},
		{
			input:    "High-quality materials (tested & approved)",
			expected: "High-quality materials (tested & approved)",
		},
		{
			input:    "Design with 中文 characters",
			expected: "Design with  characters",
		},
		{
			input:    "Features: comfort, durability & style",
			expected: "Features: comfort, durability & style",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := validator.removeUnsupportedChars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBulletPointsValidator_optimizePointFormat(t *testing.T) {
	validator := NewBulletPointsValidator()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "ergonomic design",
			expected: "Ergonomic design",
		},
		{
			input:    "High quality materials.",
			expected: "High quality materials",
		},
		{
			input:    "  multiple   spaces  ",
			expected: "Multiple spaces",
		},
		{
			input:    "Already Correct Format",
			expected: "Already Correct Format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := validator.optimizePointFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBulletPointsValidator_generateDefaultBulletPoints(t *testing.T) {
	validator := NewBulletPointsValidator()

	tests := []struct {
		productName   string
		expectedCount int
		shouldContain []string
	}{
		{
			productName:   "Gaming Chair Office Ergonomic",
			expectedCount: 4,
			shouldContain: []string{"gaming", "ergonomic", "office"},
		},
		{
			productName:   "Office Desk Chair",
			expectedCount: 4,
			shouldContain: []string{"office", "chair"},
		},
		{
			productName:   "Unknown Product",
			expectedCount: 3,
			shouldContain: []string{"quality"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.productName, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						GoodsName: tt.productName,
					},
				},
			}

			points := validator.generateDefaultBulletPoints(ctx)

			assert.Equal(t, tt.expectedCount, len(points))
			assert.LessOrEqual(t, len(points), 6) // 不超过6个要点

			// 检查是否包含预期关键词
			allText := strings.ToLower(strings.Join(points, " "))
			for _, keyword := range tt.shouldContain {
				assert.Contains(t, allText, strings.ToLower(keyword))
			}
		})
	}
}

func TestBulletPointsValidator_Handle(t *testing.T) {
	validator := NewBulletPointsValidator()

	tests := []struct {
		name         string
		bulletPoints []string
		expectError  bool
		expectPoints bool
	}{
		{
			name: "有效的要点",
			bulletPoints: []string{
				"Ergonomic design for comfort",
				"High-quality materials",
				"Easy assembly process",
			},
			expectError:  false,
			expectPoints: true,
		},
		{
			name:         "空要点列表",
			bulletPoints: []string{},
			expectError:  false,
			expectPoints: true, // 应该生成默认要点
		},
		{
			name: "需要优化的要点",
			bulletPoints: []string{
				"ergonomic design.",
				"  high quality  ",
				"easy to use",
			},
			expectError:  false,
			expectPoints: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						GoodsName: "Test Gaming Chair",
					},
					GoodsExtensionInfo: types.ExtensionInfo{
						BulletPoints: tt.bulletPoints,
					},
				},
			}

			err := validator.Handle(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.expectPoints {
					assert.NotEmpty(t, ctx.TemuProduct.GoodsExtensionInfo.BulletPoints)
					assert.LessOrEqual(t, len(ctx.TemuProduct.GoodsExtensionInfo.BulletPoints), 6)

					// 验证总长度不超过700字符
					totalLength := 0
					for _, point := range ctx.TemuProduct.GoodsExtensionInfo.BulletPoints {
						totalLength += len(point)
					}
					assert.LessOrEqual(t, totalLength, 700)
				}
			}
		})
	}
}

func TestBulletPointsValidator_calculateTotalLength(t *testing.T) {
	validator := NewBulletPointsValidator()

	points := []string{
		"Short point",                              // 11 chars
		"Medium length point here",                 // 25 chars
		"This is a longer point with more details", // 41 chars
	}

	expectedLength := 11 + 25 + 41 // 77 chars
	actualLength := validator.calculateTotalLength(points)

	assert.Equal(t, expectedLength, actualLength)
}

func TestBulletPointsValidator_truncateToLimit(t *testing.T) {
	validator := NewBulletPointsValidator()

	points := []string{
		"First point with some content",                    // 29 chars
		"Second point with more content",                   // 31 chars
		"Third point that might be too long for the limit", // 49 chars
	}

	// 测试截断到60字符限制
	result := validator.truncateToLimit(points, 60)

	// 应该包含前两个要点 (29 + 31 = 60)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, points[0], result[0])
	assert.Equal(t, points[1], result[1])

	totalLength := validator.calculateTotalLength(result)
	assert.LessOrEqual(t, totalLength, 60)
}
