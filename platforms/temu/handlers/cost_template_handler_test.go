package handlers

import (
	"context"
	"testing"

	"task-processor/common/pipeline"
	"task-processor/common/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostTemplateHandler_Handle(t *testing.T) {
	// 创建处理器
	handler := NewCostTemplateHandler()
	require.NotNil(t, handler)

	// 创建测试上下文
	task := &types.Task{
		ID:        "test-task-123",
		ProductID: "B123456789",
		StoreID:   12345,
		Region:    "US",
	}

	ctx := pipeline.NewTaskContext(context.Background(), task)

	// 初始化TEMU产品数据
	initHandler := NewInitDataHandler()
	err := initHandler.Handle(ctx)
	require.NoError(t, err)

	// 设置提交信息
	ctx.TemuProduct.GoodsBasic.ListingCommitID = "562950865107702"
	ctx.TemuProduct.GoodsBasic.GoodsCommitID = "562964090997475"
	ctx.TemuProduct.GoodsBasic.GoodsID = "602996981213936"
	ctx.TemuProduct.GoodsBasic.CatID = 12345

	// 执行处理（没有API客户端，应该使用默认模板）
	err = handler.Handle(ctx)
	require.NoError(t, err)

	// 验证默认模板被设置
	assert.Equal(t, "default_template_001", ctx.TemuProduct.GoodsServicePromise.CostTemplateID)
}

func TestCostTemplateHandler_buildCostTemplateRequest(t *testing.T) {
	handler := NewCostTemplateHandler()

	// 创建测试上下文
	task := &types.Task{
		ID:        "test-task-123",
		ProductID: "B123456789",
		StoreID:   12345,
	}

	ctx := pipeline.NewTaskContext(context.Background(), task)

	// 初始化TEMU产品数据
	initHandler := NewInitDataHandler()
	err := initHandler.Handle(ctx)
	require.NoError(t, err)

	// 设置测试数据
	ctx.TemuProduct.GoodsBasic.ListingCommitID = "test-listing-123"
	ctx.TemuProduct.GoodsBasic.GoodsCommitID = "test-goods-456"
	ctx.TemuProduct.GoodsBasic.GoodsID = "test-goods-id-789"
	ctx.TemuProduct.GoodsBasic.CatID = 12345

	// 构建请求
	request := handler.buildCostTemplateRequest(ctx)

	// 验证请求数据
	assert.Equal(t, "test-listing-123", request.ListingCommitID)
	assert.Equal(t, "test-goods-456", request.GoodsCommitID)
	assert.Equal(t, "test-goods-id-789", request.GoodsID)
	assert.Equal(t, 12345, request.CatID)
	assert.Equal(t, "8", request.ClickType)
	assert.Equal(t, "1", request.ListingCommitVersion)
	assert.Equal(t, true, request.QueryAll)
}

func TestCostTemplateHandler_extractCostTemplateID(t *testing.T) {
	handler := NewCostTemplateHandler()

	tests := []struct {
		name     string
		response *CostTemplateResponse
		expected string
	}{
		{
			name: "有默认模板",
			response: &CostTemplateResponse{
				Success:   true,
				ErrorCode: 1000000,
				Result: &CostTemplateResult{
					CostTemplateList: []CostTemplateItem{
						{
							CostTemplateID:  "template-1",
							TemplateName:    "Template 1",
							Disabled:        false,
							DefaultTemplate: false,
						},
						{
							CostTemplateID:  "default-template",
							TemplateName:    "Default Template",
							Disabled:        false,
							DefaultTemplate: true,
						},
					},
				},
			},
			expected: "default-template",
		},
		{
			name: "没有默认模板，选择第一个可用",
			response: &CostTemplateResponse{
				Success:   true,
				ErrorCode: 1000000,
				Result: &CostTemplateResult{
					CostTemplateList: []CostTemplateItem{
						{
							CostTemplateID:  "template-1",
							TemplateName:    "Template 1",
							Disabled:        false,
							DefaultTemplate: false,
						},
						{
							CostTemplateID:  "template-2",
							TemplateName:    "Template 2",
							Disabled:        false,
							DefaultTemplate: false,
						},
					},
				},
			},
			expected: "template-1",
		},
		{
			name: "所有模板都被禁用",
			response: &CostTemplateResponse{
				Success:   true,
				ErrorCode: 1000000,
				Result: &CostTemplateResult{
					CostTemplateList: []CostTemplateItem{
						{
							CostTemplateID:  "disabled-template",
							TemplateName:    "Disabled Template",
							Disabled:        true,
							DefaultTemplate: false,
						},
					},
				},
			},
			expected: "disabled-template",
		},
		{
			name: "空响应",
			response: &CostTemplateResponse{
				Success:   true,
				ErrorCode: 1000000,
				Result:    nil,
			},
			expected: "",
		},
		{
			name: "空模板列表",
			response: &CostTemplateResponse{
				Success:   true,
				ErrorCode: 1000000,
				Result: &CostTemplateResult{
					CostTemplateList: []CostTemplateItem{},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.extractCostTemplateID(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}
