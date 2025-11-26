package handlers

import (
	"strings"
	"testing"

	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/stretchr/testify/assert"
)

func TestProductDescriptionValidator_validateAndOptimizeDescription(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		name              string
		inputDescription  string
		expectValid       bool
		expectViolations  int
		expectSuggestions int
		maxExpectedLength int
	}{
		{
			name: "正常的产品描述",
			inputDescription: "This is a high-quality gaming chair designed for comfort and durability. " +
				"It features ergonomic design, adjustable height, and premium materials. " +
				"Perfect for office and gaming use.",
			expectValid:       true,
			expectViolations:  0,
			expectSuggestions: 0,
			maxExpectedLength: 10000,
		},
		{
			name:              "包含HTML标签的描述",
			inputDescription:  "<p>This is a <strong>gaming chair</strong> with <em>premium features</em>.</p>",
			expectValid:       false,
			expectViolations:  1,
			expectSuggestions: 0,
			maxExpectedLength: 10000,
		},
		{
			name:              "过短的描述",
			inputDescription:  "Good chair.",
			expectValid:       true,
			expectViolations:  0,
			expectSuggestions: 1, // 建议扩展
			maxExpectedLength: 10000,
		},
		{
			name: "包含重复句子的描述",
			inputDescription: "This chair is comfortable. This chair is durable. This chair is comfortable. " +
				"It has great features.",
			expectValid:       true,
			expectViolations:  0,
			expectSuggestions: 1, // 移除重复
			maxExpectedLength: 10000,
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
						BulletPoints: []string{"Ergonomic design", "High quality"},
					},
				},
			}

			result := validator.validateAndOptimizeDescription(tt.inputDescription, ctx)

			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectViolations, len(result.Violations))
			assert.LessOrEqual(t, result.Length, tt.maxExpectedLength)
			assert.NotEmpty(t, result.ValidatedDescription)
			assert.GreaterOrEqual(t, result.QualityScore, 0)
			assert.LessOrEqual(t, result.QualityScore, 100)
		})
	}
}

func TestProductDescriptionValidator_isValidChar(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		char     rune
		expected bool
	}{
		{'a', true},  // 字母
		{'A', true},  // 大写字母
		{'1', true},  // 数字
		{' ', true},  // 空格
		{'.', true},  // 句号
		{'!', true},  // 感叹号
		{'®', false}, // 特殊符号
		{'©', false}, // 版权符号
		{'中', false}, // 中文字符
		{'\n', true}, // 换行符
		{'\t', true}, // 制表符
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := validator.isValidChar(tt.char)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProductDescriptionValidator_convertSpecialChar(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		char     rune
		expected string
	}{
		{'®', "(R)"},
		{'©', "(C)"},
		{'™', "(TM)"},
		{'°', " degrees"},
		{'×', "x"},
		{'÷', "/"},
		{'–', "-"},
		{'—', "-"},
		{'"', "\""},
		{'"', "\""},
		{'a', ""}, // 普通字符不转换
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := validator.convertSpecialChar(tt.char)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProductDescriptionValidator_normalizeWhitespace(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Normal   text   with    spaces",
			expected: "Normal text with spaces",
		},
		{
			input:    "Text\n\n\n\nwith\nmultiple\nlines",
			expected: "Text\n\nwith\nmultiple\nlines",
		},
		{
			input:    "  Leading and trailing spaces  ",
			expected: "Leading and trailing spaces",
		},
		{
			input:    "Text\twith\ttabs",
			expected: "Text with tabs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := validator.normalizeWhitespace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProductDescriptionValidator_splitIntoSentences(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "First sentence. Second sentence! Third sentence?",
			expected: []string{"First sentence", "Second sentence", "Third sentence"},
		},
		{
			input:    "Single sentence without punctuation",
			expected: []string{"Single sentence without punctuation"},
		},
		{
			input:    "Sentence with... ellipsis.",
			expected: []string{"Sentence with", "ellipsis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := validator.splitIntoSentences(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProductDescriptionValidator_calculateQualityScore(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		name        string
		description string
		minScore    int
		maxScore    int
	}{
		{
			name: "高质量描述",
			description: "This premium gaming chair offers exceptional comfort and durability. " +
				"Features include ergonomic design, adjustable height, lumbar support, and high-quality materials. " +
				"Perfect for extended gaming sessions and office work. " +
				"The chair is built with professional-grade components for reliable performance. " +
				"Easy to assemble and maintain.",
			minScore: 70,
			maxScore: 100,
		},
		{
			name:        "中等质量描述",
			description: "Gaming chair with comfort features. Good quality materials. Adjustable height.",
			minScore:    30,
			maxScore:    70,
		},
		{
			name:        "低质量描述",
			description: "Chair.",
			minScore:    0,
			maxScore:    30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						GoodsName: "Gaming Chair",
					},
				},
			}

			score := validator.calculateQualityScore(tt.description, ctx)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
		})
	}
}

func TestProductDescriptionValidator_truncateDescription(t *testing.T) {
	validator := NewProductDescriptionValidator()

	longDescription := strings.Repeat("This is a sentence. ", 100) // 约2000字符

	result := validator.truncateDescription(longDescription, 500)

	assert.LessOrEqual(t, len(result), 500)
	assert.True(t, strings.HasSuffix(result, "...") || len(result) < len(longDescription))
}

func TestProductDescriptionValidator_generateDefaultDescription(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		productName   string
		bulletPoints  []string
		minLength     int
		shouldContain []string
	}{
		{
			productName:   "Gaming Chair Office Ergonomic",
			bulletPoints:  []string{"Comfortable seating", "Adjustable height"},
			minLength:     100,
			shouldContain: []string{"gaming chair", "comfort", "ergonomic"},
		},
		{
			productName:   "Office Desk Modern",
			bulletPoints:  []string{"Spacious surface", "Durable construction"},
			minLength:     100,
			shouldContain: []string{"desk", "workspace", "productivity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.productName, func(t *testing.T) {
			ctx := &pipeline.TaskContext{
				TemuProduct: &types.Product{
					GoodsBasic: types.GoodsBasicInfo{
						GoodsName: tt.productName,
					},
					GoodsExtensionInfo: types.ExtensionInfo{
						BulletPoints: tt.bulletPoints,
					},
				},
			}

			description := validator.generateDefaultDescription(ctx)

			assert.GreaterOrEqual(t, len(description), tt.minLength)

			descLower := strings.ToLower(description)
			for _, keyword := range tt.shouldContain {
				assert.Contains(t, descLower, strings.ToLower(keyword))
			}
		})
	}
}

func TestProductDescriptionValidator_Handle(t *testing.T) {
	validator := NewProductDescriptionValidator()

	tests := []struct {
		name        string
		description string
		expectError bool
	}{
		{
			name: "有效描述",
			description: "This is a high-quality gaming chair designed for comfort. " +
				"Features ergonomic design and premium materials.",
			expectError: false,
		},
		{
			name:        "空描述",
			description: "",
			expectError: false, // 应该生成默认描述
		},
		{
			name:        "需要清理的描述",
			description: "<p>Gaming chair with <strong>premium</strong> features.</p>",
			expectError: false,
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
						GoodsDesc:    tt.description,
						BulletPoints: []string{"Ergonomic design", "High quality"},
					},
				},
			}

			err := validator.Handle(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc)
				assert.LessOrEqual(t, len(ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc), 10000)
			}
		})
	}
}
