package handlers

import (
	"context"
	"testing"

	"task-processor/common/pipeline"
	"task-processor/common/types"
	temuTypes "task-processor/platforms/temu/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitDetailHandler_Handle(t *testing.T) {
	// 创建处理器
	handler := NewCommitDetailHandler()
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

	// 设置提交信息（模拟已创建的提交）
	ctx.TemuProduct.GoodsBasic.ListingCommitID = "562950865107702"
	ctx.TemuProduct.GoodsBasic.GoodsCommitID = "562964090997475"
	ctx.TemuProduct.GoodsBasic.GoodsID = "602996981213936"
	ctx.TemuProduct.GoodsBasic.ListingCommitVersion = "1"

	// 注意：这个测试不会实际调用API，因为没有设置APIClient
	// 在实际环境中，需要mock APIClient来测试完整流程
	err = handler.Handle(ctx)

	// 由于没有APIClient，应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API客户端未初始化")
}

func TestCommitDetailHandler_validateCommitInfo(t *testing.T) {
	handler := NewCommitDetailHandler()

	tests := []struct {
		name        string
		setupCtx    func() *pipeline.TaskContext
		expectError bool
		errorMsg    string
	}{
		{
			name: "有效的提交信息",
			setupCtx: func() *pipeline.TaskContext {
				task := &types.Task{StoreID: 12345}
				ctx := pipeline.NewTaskContext(context.Background(), task)
				ctx.TemuProduct = &temuTypes.Product{}
				ctx.TemuProduct.GoodsBasic.ListingCommitID = "123"
				ctx.TemuProduct.GoodsBasic.GoodsCommitID = "456"
				ctx.TemuProduct.GoodsBasic.GoodsID = "789"
				ctx.TemuProduct.GoodsBasic.ListingCommitVersion = "1"
				return ctx
			},
			expectError: false,
		},
		{
			name: "ListingCommitID为空",
			setupCtx: func() *pipeline.TaskContext {
				task := &types.Task{StoreID: 12345}
				ctx := pipeline.NewTaskContext(context.Background(), task)
				ctx.TemuProduct = &temuTypes.Product{}
				ctx.TemuProduct.GoodsBasic.ListingCommitID = ""
				ctx.TemuProduct.GoodsBasic.GoodsCommitID = "456"
				ctx.TemuProduct.GoodsBasic.GoodsID = "789"
				ctx.TemuProduct.GoodsBasic.ListingCommitVersion = "1"
				return ctx
			},
			expectError: true,
			errorMsg:    "ListingCommitID不能为空",
		},
		{
			name: "GoodsCommitID为空",
			setupCtx: func() *pipeline.TaskContext {
				task := &types.Task{StoreID: 12345}
				ctx := pipeline.NewTaskContext(context.Background(), task)
				ctx.TemuProduct = &temuTypes.Product{}
				ctx.TemuProduct.GoodsBasic.ListingCommitID = "123"
				ctx.TemuProduct.GoodsBasic.GoodsCommitID = ""
				ctx.TemuProduct.GoodsBasic.GoodsID = "789"
				ctx.TemuProduct.GoodsBasic.ListingCommitVersion = "1"
				return ctx
			},
			expectError: true,
			errorMsg:    "GoodsCommitID不能为空",
		},
		{
			name: "GoodsID为空",
			setupCtx: func() *pipeline.TaskContext {
				task := &types.Task{StoreID: 12345}
				ctx := pipeline.NewTaskContext(context.Background(), task)
				ctx.TemuProduct = &temuTypes.Product{}
				ctx.TemuProduct.GoodsBasic.ListingCommitID = "123"
				ctx.TemuProduct.GoodsBasic.GoodsCommitID = "456"
				ctx.TemuProduct.GoodsBasic.GoodsID = ""
				ctx.TemuProduct.GoodsBasic.ListingCommitVersion = "1"
				return ctx
			},
			expectError: true,
			errorMsg:    "GoodsID不能为空",
		},
		{
			name: "ListingCommitVersion为空",
			setupCtx: func() *pipeline.TaskContext {
				task := &types.Task{StoreID: 12345}
				ctx := pipeline.NewTaskContext(context.Background(), task)
				ctx.TemuProduct = &temuTypes.Product{}
				ctx.TemuProduct.GoodsBasic.ListingCommitID = "123"
				ctx.TemuProduct.GoodsBasic.GoodsCommitID = "456"
				ctx.TemuProduct.GoodsBasic.GoodsID = "789"
				ctx.TemuProduct.GoodsBasic.ListingCommitVersion = ""
				return ctx
			},
			expectError: true,
			errorMsg:    "ListingCommitVersion不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			err := handler.validateCommitInfo(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommitDetailHandler_updateProductFromCommitDetail(t *testing.T) {
	handler := NewCommitDetailHandler()

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

	// 创建测试的提交详情数据
	result := &CommitDetailResult{
		GoodsBasic: &CommitDetailGoodsBasic{
			GoodsName:   "Test Product Name",
			CatID:       33463,
			CatIDs:      []int{31148, 32875, 33273, 33429, 33430, 33463},
			GoodsType:   1,
			IsClothes:   false,
			IsBooks:     false,
			Customized:  false,
			SecondHand:  false,
			MadeToOrder: false,
			OutGoodsSn:  "TEST-SKU-001",
			CategoryTree: &CommitDetailCategoryTree{
				Level:        6,
				CateType:     1,
				CatID:        33463,
				Cate1ID:      31148,
				Cate1Name:    "运动与户外用品",
				Cate2ID:      32875,
				Cate2Name:    "户外休闲",
				Cate3ID:      33273,
				Cate3Name:    "骑行用品",
				Cate4ID:      33429,
				Cate4Name:    "服装",
				Cate5ID:      33430,
				Cate5Name:    "男士骑行服",
				CateNameList: []string{"运动与户外用品", "户外休闲", "骑行用品", "服装", "男士骑行服", "男士骑行雨披"},
			},
			CategoryDisclaimer: &CommitDetailCategoryDisclaimer{
				PromptList: []string{"测试免责声明1", "测试免责声明2"},
			},
		},
		GoodsSaleInfo: &CommitDetailGoodsSaleInfo{
			GoodsPattern: 11,
		},
		Extra: &CommitDetailExtra{
			Tab:             1,
			MinSkuImageSize: 1,
			MaxSkuImageSize: 10,
		},
		CanSave:               true,
		SupportMaxRetailPrice: true,
		PlatformExpressBill:   false,
	}

	// 执行数据更新
	err = handler.updateProductFromCommitDetail(ctx, result)
	require.NoError(t, err)

	// 验证基础信息更新
	assert.Equal(t, "Test Product Name", ctx.TemuProduct.GoodsBasic.GoodsName)
	assert.Equal(t, 33463, ctx.TemuProduct.GoodsBasic.CatID)
	assert.Equal(t, []int{31148, 32875, 33273, 33429, 33430, 33463}, ctx.TemuProduct.GoodsBasic.CatIDs)
	assert.Equal(t, 1, ctx.TemuProduct.GoodsBasic.GoodsType)
	assert.Equal(t, false, ctx.TemuProduct.GoodsBasic.IsClothes)
	assert.Equal(t, false, ctx.TemuProduct.GoodsBasic.IsBooks)
	assert.Equal(t, false, ctx.TemuProduct.GoodsBasic.Customized)
	assert.Equal(t, false, ctx.TemuProduct.GoodsBasic.SecondHand)
	assert.Equal(t, false, ctx.TemuProduct.GoodsBasic.MadeToOrder)
	assert.Equal(t, "TEST-SKU-001", ctx.TemuProduct.GoodsBasic.OutGoodsSN)

	// 验证分类树信息更新
	assert.Equal(t, 6, ctx.TemuProduct.GoodsBasic.CategoryTree.Level)
	assert.Equal(t, 1, ctx.TemuProduct.GoodsBasic.CategoryTree.CateType)
	assert.Equal(t, 33463, ctx.TemuProduct.GoodsBasic.CategoryTree.CatID)
	assert.Equal(t, 31148, ctx.TemuProduct.GoodsBasic.CategoryTree.Cate1ID)
	assert.Equal(t, "运动与户外用品", ctx.TemuProduct.GoodsBasic.CategoryTree.Cate1Name)
	assert.Equal(t, 32875, ctx.TemuProduct.GoodsBasic.CategoryTree.Cate2ID)
	assert.Equal(t, "户外休闲", ctx.TemuProduct.GoodsBasic.CategoryTree.Cate2Name)
	assert.Equal(t, []string{"运动与户外用品", "户外休闲", "骑行用品", "服装", "男士骑行服", "男士骑行雨披"}, ctx.TemuProduct.GoodsBasic.CategoryTree.CateNameList)

	// 验证分类免责声明更新
	assert.Equal(t, []string{"测试免责声明1", "测试免责声明2"}, ctx.TemuProduct.GoodsBasic.CategoryDisclaimer.PromptList)

	// 验证销售信息更新
	assert.Equal(t, 0, ctx.TemuProduct.GoodsSaleInfo.GoodsPattern)

	// 验证额外信息更新
	assert.Equal(t, 1, ctx.TemuProduct.Extra.Tab)
	assert.Equal(t, 1, ctx.TemuProduct.Extra.MinSkuImageSize)
	assert.Equal(t, 10, ctx.TemuProduct.Extra.MaxSkuImageSize)

	// 验证支持标志更新
	assert.NotNil(t, ctx.TemuProduct.CanSave)
	assert.Equal(t, true, *ctx.TemuProduct.CanSave)
	assert.NotNil(t, ctx.TemuProduct.SupportMaxRetailPrice)
	assert.Equal(t, true, *ctx.TemuProduct.SupportMaxRetailPrice)
	assert.NotNil(t, ctx.TemuProduct.PlatformExpressBill)
	assert.Equal(t, false, *ctx.TemuProduct.PlatformExpressBill)
}

func TestGetCommitDetailFromContext_CommitDetailHandler(t *testing.T) {
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
