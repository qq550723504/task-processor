// Package scheduler 提供SHEIN平台SKU映射关系修复功能测试
package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	managementapi "task-processor/internal/pkg/management/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMappingClient 模拟映射客户端
type MockMappingClient struct {
	mock.Mock
}

func (m *MockMappingClient) CreateProductImportMapping(req *managementapi.ProductImportMappingCreateReqDTO) (int64, error) {
	args := m.Called(req)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMappingClient) GetProductImportMappingByPlatformProductIdAndStore(req *managementapi.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*managementapi.ProductImportMappingRespDTO, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*managementapi.ProductImportMappingRespDTO), args.Error(1)
}

func (m *MockMappingClient) GetProductImportMappingByPlatformProductId(req *managementapi.ProductImportMappingGetReqDTO) (*managementapi.ProductImportMappingRespDTO, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*managementapi.ProductImportMappingRespDTO), args.Error(1)
}

func (m *MockMappingClient) CheckProductExists(req *managementapi.ProductImportMappingCheckReqDTO) (bool, error) {
	args := m.Called(req)
	return args.Bool(0), args.Error(1)
}

func (m *MockMappingClient) GetProductImportMappingBySku(req *managementapi.ProductImportMappingGetBySkuReqDTO) (*managementapi.ProductImportMappingRespDTO, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*managementapi.ProductImportMappingRespDTO), args.Error(1)
}

func (m *MockMappingClient) GetProductImportMappingByTaskAndSku(importTaskId int64, sku string) (*managementapi.ProductImportMappingRespDTO, error) {
	args := m.Called(importTaskId, sku)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*managementapi.ProductImportMappingRespDTO), args.Error(1)
}

func (m *MockMappingClient) UpdateProductImportMapping(req *managementapi.ProductImportMappingCreateReqDTO) error {
	args := m.Called(req)
	return args.Error(0)
}

// MockStoreAPI 模拟店铺API
type MockStoreAPI struct {
	mock.Mock
}

func (m *MockStoreAPI) GetStore(storeID int64) (*managementapi.StoreRespDTO, error) {
	args := m.Called(storeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*managementapi.StoreRespDTO), args.Error(1)
}

func (m *MockStoreAPI) GetStoreCookie(id int64) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockStoreAPI) UpdateStoreId(req *managementapi.StoreIdUpdateReqDTO) (bool, error) {
	args := m.Called(req)
	return args.Bool(0), args.Error(1)
}

func (m *MockStoreAPI) UpdateStoreStatus(req *managementapi.StoreStatusUpdateReqDTO) (bool, error) {
	args := m.Called(req)
	return args.Bool(0), args.Error(1)
}

func (m *MockStoreAPI) DeleteStoreCookie(id int64) (bool, error) {
	args := m.Called(id)
	return args.Bool(0), args.Error(1)
}

func (m *MockStoreAPI) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	args := m.Called(id, pause, pauseType)
	return args.Bool(0), args.Error(1)
}

// TestMappingRepairConfig 测试修复配置
func TestMappingRepairConfig(t *testing.T) {
	config := DefaultMappingRepairConfig()

	assert.Equal(t, 3, config.MaxRetryCount)
	assert.Equal(t, 5*time.Minute, config.RetryInterval)
	assert.Equal(t, 50, config.BatchSize)
	assert.True(t, config.EnableAutoRepair)
	assert.Equal(t, 30*time.Second, config.RepairTimeout)
}

// TestMappingRepairRequest 测试修复请求创建
func TestMappingRepairRequest(t *testing.T) {
	// 测试从错误创建修复请求
	request := CreateRepairRequestFromError(
		"TEST_SKU_001",
		100,
		1,
		assert.AnError,
		"TEST_SPU_001",
		"测试产品",
	)

	assert.Equal(t, "TEST_SKU_001", request.SkuCode)
	assert.Equal(t, int64(100), request.StoreID)
	assert.Equal(t, int64(1), request.TenantID)
	assert.Equal(t, "TEST_SPU_001", request.SpuCode)
	assert.Equal(t, "测试产品", request.SpuName)
	assert.Equal(t, 2, request.Priority)
	assert.Contains(t, request.Reason, "查询映射关系失败")
}

// TestMappingRepairService 测试修复服务
func TestMappingRepairService(t *testing.T) {
	mockMappingClient := &MockMappingClient{}
	mockStoreAPI := &MockStoreAPI{}

	// 设置mock期望
	mockStoreAPI.On("GetStore", int64(100)).Return(&managementapi.StoreRespDTO{
		ID:     100,
		Region: "US",
	}, nil)

	mockMappingClient.On("CreateProductImportMapping", mock.AnythingOfType("*api.ProductImportMappingCreateReqDTO")).Return(int64(1), nil)

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.AnythingOfType("*api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO")).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                1,
		PlatformProductId: stringPtr("TEST_SKU_001"),
		StoreId:           100,
	}, nil)

	// 创建修复服务
	service := NewMappingRepairService(
		mockMappingClient,
		mockStoreAPI,
		nil, // productAPI
		DefaultMappingRepairConfig(),
	)

	// 测试单个修复
	request := &MappingRepairRequest{
		TenantID: 1,
		StoreID:  100,
		SkuCode:  "TEST_SKU_001",
		SpuCode:  "TEST_SPU_001",
		Reason:   "测试修复",
		Priority: 1,
	}

	ctx := context.Background()
	result, err := service.RepairMapping(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "TEST_SKU_001", result.SkuCode)

	// 验证mock调用
	mockStoreAPI.AssertExpectations(t)
	mockMappingClient.AssertExpectations(t)
}

// TestMappingRepairStats 测试修复统计
func TestMappingRepairStats(t *testing.T) {
	mockMappingClient := &MockMappingClient{}
	mockStoreAPI := &MockStoreAPI{}

	service := NewMappingRepairService(
		mockMappingClient,
		mockStoreAPI,
		nil,
		DefaultMappingRepairConfig(),
	)

	stats := service.GetRepairStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.SuccessCount)
	assert.Equal(t, int64(0), stats.FailedCount)
}

// TestProductBasedRepairStrategy 测试基于产品信息的修复策略
func TestProductBasedRepairStrategy(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	strategy := NewProductBasedRepairStrategy(mockMappingClient, nil)

	// 测试策略名称
	assert.Equal(t, "ProductBasedRepair", strategy.GetStrategyName())

	// 测试是否可以修复
	ctx := &MappingRepairContext{
		Request: &MappingRepairRequest{
			SkuCode: "TEST_SKU_001",
		},
		StoreInfo: &managementapi.StoreRespDTO{
			ID:     100,
			Region: "US",
		},
	}

	assert.True(t, strategy.CanRepair(ctx))

	// 测试没有SKU编码的情况
	ctxNoSku := &MappingRepairContext{
		Request: &MappingRepairRequest{
			SkuCode: "",
		},
		StoreInfo: &managementapi.StoreRespDTO{
			ID: 100,
		},
	}

	assert.False(t, strategy.CanRepair(ctxNoSku))
}

// TestHistoryBasedRepairStrategy 测试基于历史记录的修复策略
func TestHistoryBasedRepairStrategy(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	strategy := NewHistoryBasedRepairStrategy(mockMappingClient)

	// 测试策略名称
	assert.Equal(t, "HistoryBasedRepair", strategy.GetStrategyName())

	// 测试是否可以修复
	ctx := &MappingRepairContext{
		Request: &MappingRepairRequest{
			SpuCode: "TEST_SPU_001",
		},
	}

	assert.True(t, strategy.CanRepair(ctx))

	// 测试没有SPU信息的情况
	ctxNoSpu := &MappingRepairContext{
		Request: &MappingRepairRequest{
			SpuCode: "",
			SpuName: "",
		},
	}

	assert.False(t, strategy.CanRepair(ctxNoSpu))
}

// TestMappingRepairHandler 测试修复处理器
func TestMappingRepairHandler(t *testing.T) {
	mockMappingClient := &MockMappingClient{}
	mockStoreAPI := &MockStoreAPI{}

	service := NewMappingRepairService(
		mockMappingClient,
		mockStoreAPI,
		nil,
		DefaultMappingRepairConfig(),
	)

	handler := NewMappingRepairHandler(service, DefaultMappingRepairConfig(), 1)

	assert.NotNil(t, handler)
	assert.Equal(t, 1, handler.workers)
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}

// BenchmarkMappingRepair 性能测试
func BenchmarkMappingRepair(b *testing.B) {
	mockMappingClient := &MockMappingClient{}
	mockStoreAPI := &MockStoreAPI{}

	// 设置mock期望
	mockStoreAPI.On("GetStore", mock.AnythingOfType("int64")).Return(&managementapi.StoreRespDTO{
		ID:     100,
		Region: "US",
	}, nil)

	mockMappingClient.On("CreateProductImportMapping", mock.AnythingOfType("*api.ProductImportMappingCreateReqDTO")).Return(int64(1), nil)

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.AnythingOfType("*api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO")).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                1,
		PlatformProductId: stringPtr("TEST_SKU"),
		StoreId:           100,
	}, nil)

	service := NewMappingRepairService(
		mockMappingClient,
		mockStoreAPI,
		nil,
		DefaultMappingRepairConfig(),
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := &MappingRepairRequest{
			TenantID: 1,
			StoreID:  100,
			SkuCode:  "TEST_SKU",
			Reason:   "性能测试",
			Priority: 1,
		}

		_, err := service.RepairMapping(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestMappingBuilder 测试映射关系构建器
func TestMappingBuilder(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	// 设置mock期望
	mockMappingClient.On("CreateProductImportMapping", mock.AnythingOfType("*api.ProductImportMappingCreateReqDTO")).Return(int64(1), nil)

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.AnythingOfType("*api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO")).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                1,
		PlatformProductId: stringPtr("TEST_SKU_001"),
		StoreId:           100,
	}, nil)

	// 创建映射构建器
	builder := NewMappingBuilder(mockMappingClient)

	// 测试基础映射创建
	result, err := builder.CreateBasicMapping(1, 100, "TEST_SKU_001", "US", "测试创建")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "TEST_SKU_001", *result.PlatformProductId)

	// 验证mock调用
	mockMappingClient.AssertExpectations(t)
}

// TestMappingBuilderWithSPU 测试包含SPU信息的映射创建
func TestMappingBuilderWithSPU(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	// 设置mock期望
	mockMappingClient.On("CreateProductImportMapping", mock.AnythingOfType("*api.ProductImportMappingCreateReqDTO")).Return(int64(2), nil)

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.AnythingOfType("*api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO")).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                2,
		PlatformProductId: stringPtr("TEST_SKU_002"),
		StoreId:           100,
	}, nil)

	// 创建映射构建器
	builder := NewMappingBuilder(mockMappingClient)

	// 测试包含SPU信息的映射创建
	result, err := builder.CreateMappingWithSPU(1, 100, "TEST_SKU_002", "TEST_SPU_002", "测试产品002", "US", "测试SPU映射创建")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(2), result.ID)

	// 验证mock调用
	mockMappingClient.AssertExpectations(t)
}

// TestMappingBuilderValidation 测试映射构建器参数验证
func TestMappingBuilderValidation(t *testing.T) {
	mockMappingClient := &MockMappingClient{}
	builder := NewMappingBuilder(mockMappingClient)

	// 测试无效的租户ID
	_, err := builder.CreateBasicMapping(0, 100, "TEST_SKU", "US", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "租户ID不能为空")

	// 测试无效的店铺ID
	_, err = builder.CreateBasicMapping(1, 0, "TEST_SKU", "US", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "店铺ID不能为空")

	// 测试空的SKU编码
	_, err = builder.CreateBasicMapping(1, 100, "", "US", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SKU编码不能为空")

	// 测试空的区域
	_, err = builder.CreateBasicMapping(1, 100, "TEST_SKU", "", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "区域不能为空")
}

// TestMappingBuilderWithRules 测试包含规则信息的映射创建
func TestMappingBuilderWithRules(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	// 设置mock期望
	profitRuleID := int64(10)
	filterRuleID := int64(20)
	salePriceMultiplier := "1.5"
	discountPriceMultiplier := "1.2"
	filterRuleRange := "10.00-100.00"

	mockMappingClient.On("CreateProductImportMapping", mock.MatchedBy(func(req *managementapi.ProductImportMappingCreateReqDTO) bool {
		return req.ProfitRuleId != nil && *req.ProfitRuleId == profitRuleID &&
			req.FilterRuleId != nil && *req.FilterRuleId == filterRuleID
	})).Return(int64(3), nil)

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.AnythingOfType("*api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO")).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                3,
		PlatformProductId: stringPtr("TEST_SKU_003"),
		StoreId:           100,
	}, nil)

	// 创建映射构建器
	builder := NewMappingBuilder(mockMappingClient)

	// 测试包含规则信息的映射创建
	result, err := builder.CreateMappingWithRules(
		1, 100, "TEST_SKU_003", "US", "测试规则映射创建",
		&profitRuleID, &filterRuleID,
		&salePriceMultiplier, &discountPriceMultiplier, &filterRuleRange,
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(3), result.ID)

	// 验证mock调用
	mockMappingClient.AssertExpectations(t)
}

// TestMappingBuilderFromTaskContext 测试从任务上下文创建映射关系
func TestMappingBuilderFromTaskContext(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	// 设置mock期望
	parentProductID := "PARENT_ASIN_001"
	platformParentProductID := "SPU_NAME_001"
	costPrice := 25.99

	mockMappingClient.On("CreateProductImportMapping", mock.MatchedBy(func(req *managementapi.ProductImportMappingCreateReqDTO) bool {
		return req.ProductId == "ASIN_001" &&
			req.ParentProductId != nil && *req.ParentProductId == parentProductID &&
			req.PlatformParentProductId != nil && *req.PlatformParentProductId == platformParentProductID &&
			req.CostPrice != nil && *req.CostPrice == costPrice
	})).Return(int64(4), nil)

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.AnythingOfType("*api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO")).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                4,
		PlatformProductId: stringPtr("TEST_SKU_004"),
		StoreId:           100,
	}, nil)

	// 创建映射构建器
	builder := NewMappingBuilder(mockMappingClient)

	// 测试从任务上下文创建映射关系
	result, err := builder.CreateMappingFromTaskContext(
		1, 100, "TEST_SKU_004", "SUPPLIER_SKU_004", "ASIN_001", "US", "测试任务上下文映射创建",
		&parentProductID, &platformParentProductID, &costPrice,
		nil, nil, nil, nil, nil,
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(4), result.ID)

	// 验证mock调用
	mockMappingClient.AssertExpectations(t)
}

// TestBatchCreateMappings 测试批量创建映射关系
func TestBatchCreateMappings(t *testing.T) {
	mockMappingClient := &MockMappingClient{}

	// 设置mock期望 - 第一个成功，第二个失败
	mockMappingClient.On("CreateProductImportMapping", mock.MatchedBy(func(req *managementapi.ProductImportMappingCreateReqDTO) bool {
		return req.ProductId == "BATCH_SKU_001"
	})).Return(int64(1), nil)

	mockMappingClient.On("CreateProductImportMapping", mock.MatchedBy(func(req *managementapi.ProductImportMappingCreateReqDTO) bool {
		return req.ProductId == "BATCH_SKU_002"
	})).Return(int64(0), fmt.Errorf("创建失败"))

	mockMappingClient.On("GetProductImportMappingByPlatformProductIdAndStore", mock.MatchedBy(func(req *managementapi.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) bool {
		return req.PlatformProductId == "BATCH_SKU_001"
	})).Return(&managementapi.ProductImportMappingRespDTO{
		ID:                1,
		PlatformProductId: stringPtr("BATCH_SKU_001"),
		StoreId:           100,
	}, nil)

	builder := NewMappingBuilder(mockMappingClient)

	// 准备批量创建选项
	optionsList := []*MappingCreateOptions{
		{
			TenantID: 1,
			StoreID:  100,
			SkuCode:  "BATCH_SKU_001",
			Region:   "US",
			Reason:   "批量测试1",
		},
		{
			TenantID: 1,
			StoreID:  100,
			SkuCode:  "BATCH_SKU_002",
			Region:   "US",
			Reason:   "批量测试2",
		},
	}

	// 执行批量创建
	results, errors := builder.BatchCreateMappings(optionsList)

	// 验证结果
	assert.Len(t, results, 2)
	assert.Len(t, errors, 2)

	// 第一个应该成功
	assert.NoError(t, errors[0])
	assert.NotNil(t, results[0])
	assert.Equal(t, int64(1), results[0].ID)

	// 第二个应该失败
	assert.Error(t, errors[1])
	assert.Nil(t, results[1])

	// 验证mock调用
	mockMappingClient.AssertExpectations(t)
}
