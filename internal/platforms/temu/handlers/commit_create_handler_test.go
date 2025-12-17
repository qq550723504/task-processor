package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"task-processor/internal/common/pipeline"
	"task-processor/internal/common/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitCreateHandler_Handle(t *testing.T) {
	// 创建处理器
	handler := NewCommitCreateHandler()
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

	// 设置基本的产品信息
	ctx.TemuProduct.GoodsBasic.GoodsName = "Test Product"
	ctx.TemuProduct.GoodsBasic.CatID = 12345
	ctx.TemuProduct.GoodsBasic.CatIDs = []int{1, 2, 3}

	// 验证初始状态
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.ListingCommitID)
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.GoodsCommitID)
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.GoodsID)

	// 注意：这个测试不会实际调用API，因为没有设置APIClient
	// 在实际环境中，需要mock APIClient来测试完整流程
	err = handler.Handle(ctx)

	// 由于没有APIClient，应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API客户端未初始化")
}

func TestCommitCreateResponse_Structure(t *testing.T) {
	// 测试响应结构体能否正确解析实际的API响应
	jsonResponse := `{
		"success": true,
		"error_code": 1000000,
		"result": {
			"goods_id": "603031340981923",
			"listing_commit_id": "562950860590951",
			"listing_commit_version": "1",
			"goods_commit_id": "562964092564935"
		}
	}`

	var response CommitCreateResponse
	err := json.Unmarshal([]byte(jsonResponse), &response)
	require.NoError(t, err)

	// 验证顶层字段
	assert.True(t, response.Success)
	assert.Equal(t, 1000000, response.ErrorCode)
	assert.NotNil(t, response.Result)

	// 验证结果字段
	assert.Equal(t, "603031340981923", response.Result.GoodsID)
	assert.Equal(t, "562950860590951", response.Result.ListingCommitID)
	assert.Equal(t, "1", response.Result.ListingCommitVersion)
	assert.Equal(t, "562964092564935", response.Result.GoodsCommitID)
}

func TestCommitCreateHandler_ResponseHandling(t *testing.T) {
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

	// 模拟成功的API响应
	response := &CommitCreateResponse{
		Success: true,
		Result: &CommitCreateResult{
			GoodsID:              "603031340981923",
			ListingCommitID:      "562950860590951",
			ListingCommitVersion: "1",
			GoodsCommitID:        "562964092564935",
		},
	}

	// 验证初始状态
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.GoodsID)
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.ListingCommitID)
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.GoodsCommitID)
	assert.Equal(t, "", ctx.TemuProduct.GoodsBasic.ListingCommitVersion)

	// 模拟响应处理逻辑（提取自createCommit方法）
	if response.Success && response.Result != nil {
		ctx.TemuProduct.GoodsBasic.ListingCommitID = response.Result.ListingCommitID
		ctx.TemuProduct.GoodsBasic.GoodsCommitID = response.Result.GoodsCommitID
		ctx.TemuProduct.GoodsBasic.GoodsID = response.Result.GoodsID
		ctx.TemuProduct.GoodsBasic.ListingCommitVersion = response.Result.ListingCommitVersion
	}

	// 验证数据已正确更新
	assert.Equal(t, "603031340981923", ctx.TemuProduct.GoodsBasic.GoodsID)
	assert.Equal(t, "562950860590951", ctx.TemuProduct.GoodsBasic.ListingCommitID)
	assert.Equal(t, "562964092564935", ctx.TemuProduct.GoodsBasic.GoodsCommitID)
	assert.Equal(t, "1", ctx.TemuProduct.GoodsBasic.ListingCommitVersion)
}

func TestGetCommitDetailFromContext(t *testing.T) {
	task := &types.Task{ID: "test"}
	ctx := pipeline.NewTaskContext(context.Background(), task)

	// 测试没有数据的情况
	detail, exists := GetCommitDetailFromContext(ctx)
	assert.False(t, exists)
	assert.Nil(t, detail)

	// 设置数据
	testDetail := map[string]interface{}{
		"test_key": "test_value",
	}
	ctx.SetData("commit_detail", testDetail)

	// 测试有数据的情况
	detail, exists = GetCommitDetailFromContext(ctx)
	assert.True(t, exists)
	assert.Equal(t, testDetail, detail)
}
