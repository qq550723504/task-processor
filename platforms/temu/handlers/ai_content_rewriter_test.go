package handlers

import (
	"testing"

	"task-processor/common/amazon/model"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestBuildSystemPrompt(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil)

	prompt := rewriter.buildSystemPrompt()

	// 验证包含核心要求
	assert.Contains(t, prompt, "TEMU平台")
	assert.Contains(t, prompt, "标题要求")
	assert.Contains(t, prompt, "描述要求")
	assert.Contains(t, prompt, "要点要求")

	// 验证包含儿童产品约束
	assert.Contains(t, prompt, "不要添加儿童相关的描述")
	assert.Contains(t, prompt, "不要提及\"适合儿童\"")
	assert.Contains(t, prompt, "聚焦于成人或通用使用场景")

	// 验证输出格式要求
	assert.Contains(t, prompt, "JSON格式")
	assert.Contains(t, prompt, "title")
	assert.Contains(t, prompt, "description")
	assert.Contains(t, prompt, "bullet_points")
}

func TestBuildUserPrompt(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil)

	product := &model.Product{
		Title:             "Ergonomic Office Chair",
		Brand:             "Test Brand",
		Description:       "High quality office chair",
		Features:          []string{"Adjustable height", "Lumbar support"},
		ProductDimensions: "24 x 24 x 40 inches",
		ItemWeight:        "35 pounds",
		ModelNumber:       "OC-2024",
		Department:        "Office Products",
	}

	ctx := &pipeline.TaskContext{
		AmazonProduct: product,
	}

	prompt := rewriter.buildUserPrompt(ctx)

	// 验证包含产品信息
	assert.Contains(t, prompt, "Ergonomic Office Chair")
	assert.Contains(t, prompt, "Test Brand")
	assert.Contains(t, prompt, "High quality office chair")
	assert.Contains(t, prompt, "Adjustable height")
	assert.Contains(t, prompt, "24 x 24 x 40 inches")
	assert.Contains(t, prompt, "35 pounds")

	// 验证包含任务说明
	assert.Contains(t, prompt, "任务")
	assert.Contains(t, prompt, "不要在标题、描述或要点中添加任何儿童相关的词汇")
}

func TestCleanJSONContent(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "带json标记的代码块",
			input:    "```json\n{\"test\": \"value\"}\n```",
			expected: "{\"test\": \"value\"}",
		},
		{
			name:     "带普通标记的代码块",
			input:    "```\n{\"test\": \"value\"}\n```",
			expected: "{\"test\": \"value\"}",
		},
		{
			name:     "纯JSON",
			input:    "{\"test\": \"value\"}",
			expected: "{\"test\": \"value\"}",
		},
		{
			name:     "带空格的JSON",
			input:    "  {\"test\": \"value\"}  ",
			expected: "{\"test\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriter.cleanJSONContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyRewriteResult(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil)

	ctx := &pipeline.TaskContext{
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				GoodsName: "Original Title",
			},
			GoodsExtensionInfo: types.ExtensionInfo{
				GoodsDesc:    "Original Description",
				BulletPoints: []string{"Point 1", "Point 2"},
			},
		},
	}

	result := &RewriteResult{
		Title:        "New Title",
		Description:  "New Description",
		BulletPoints: []string{"New Point 1", "New Point 2", "New Point 3"},
	}

	rewriter.applyRewriteResult(ctx, result)

	assert.Equal(t, "New Title", ctx.TemuProduct.GoodsBasic.GoodsName)
	assert.Equal(t, "New Description", ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc)
	assert.Equal(t, 3, len(ctx.TemuProduct.GoodsExtensionInfo.BulletPoints))
	assert.Equal(t, "New Point 1", ctx.TemuProduct.GoodsExtensionInfo.BulletPoints[0])
}

func TestHandleWithoutOpenAI(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil) // nil表示没有OpenAI客户端

	ctx := &pipeline.TaskContext{
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				GoodsName: "Original Title",
			},
		},
		AmazonProduct: &model.Product{
			Title: "Test Product",
		},
	}

	// 应该不返回错误，只是跳过AI重构
	err := rewriter.Handle(ctx)
	assert.NoError(t, err)

	// 标题应该保持不变
	assert.Equal(t, "Original Title", ctx.TemuProduct.GoodsBasic.GoodsName)
}

func TestApplyRewriteResultWithNilResult(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil)

	ctx := &pipeline.TaskContext{
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				GoodsName: "Original Title",
			},
		},
	}

	// 传入nil结果，应该不会panic
	rewriter.applyRewriteResult(ctx, nil)

	// 标题应该保持不变
	assert.Equal(t, "Original Title", ctx.TemuProduct.GoodsBasic.GoodsName)
}

func TestApplyRewriteResultWithEmptyFields(t *testing.T) {
	logger := logrus.WithField("test", "ai_content_rewriter")
	rewriter := NewAIContentRewriter(logger, nil)

	ctx := &pipeline.TaskContext{
		TemuProduct: &types.Product{
			GoodsBasic: types.GoodsBasicInfo{
				GoodsName: "Original Title",
			},
			GoodsExtensionInfo: types.ExtensionInfo{
				GoodsDesc:    "Original Description",
				BulletPoints: []string{"Point 1"},
			},
		},
	}

	// 空字段的结果
	result := &RewriteResult{
		Title:        "",
		Description:  "",
		BulletPoints: []string{},
	}

	rewriter.applyRewriteResult(ctx, result)

	// 原始内容应该保持不变
	assert.Equal(t, "Original Title", ctx.TemuProduct.GoodsBasic.GoodsName)
	assert.Equal(t, "Original Description", ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc)
	assert.Equal(t, 1, len(ctx.TemuProduct.GoodsExtensionInfo.BulletPoints))
}
