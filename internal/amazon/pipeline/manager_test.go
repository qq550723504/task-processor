// package pipeline 提供Amazon处理器管理功能测试
package pipeline

import (
	"context"
	"task-processor/internal/amazon/model"
	"task-processor/internal/core/logger"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandlerManager_ProcessProduct 测试产品处理流程
func TestHandlerManager_ProcessProduct(t *testing.T) {
	// 创建服务容器
	services := model.NewServices()

	// 创建处理器管理器
	manager := NewHandlerManager(services)

	// 验证处理器数量
	assert.Equal(t, 12, manager.GetHandlerCount())

	// 创建测试任务上下文
	taskContext := &model.TaskContext{
		TaskID:        "test-001",
		MarketplaceID: "ATVPDKIKX0DER",
		LanguageTag:   "en_US",
		Currency:      "USD",
		Data: map[string]any{
			"product_id":    "test-product-123",
			"store_id":      int64(1),
			"tenant_id":     int64(1),
			"tenantId":      int64(1),
			"storeId":       int64(1),
			"raw_json_data": `{"title":"Test Product","price":29.99}`,
		},
	}

	// 执行处理流程
	ctx := context.Background()
	err := manager.ProcessProduct(ctx, taskContext)

	// 验证处理结果
	assert.NoError(t, err)
	assert.NotNil(t, taskContext.Results)

	// 验证关键结果
	productType, exists := taskContext.GetResult("product_type")
	assert.True(t, exists)
	assert.NotEmpty(t, productType)

	listingSKU, exists := taskContext.GetResult("listing_sku")
	assert.True(t, exists)
	assert.NotEmpty(t, listingSKU)
}

// TestHandlerManager_GetStatus 测试获取状态
func TestHandlerManager_GetStatus(t *testing.T) {
	services := model.NewServices()
	manager := NewHandlerManager(services)

	status := manager.GetStatus()

	assert.Equal(t, 12, status["total_handlers"])
	assert.NotNil(t, status["handlers"])

	handlers := status["handlers"].([]map[string]any)
	assert.Len(t, handlers, 12)

	// 验证第一个处理器
	firstHandler := handlers[0]
	assert.Equal(t, "ready", firstHandler["status"])
	assert.NotEmpty(t, firstHandler["name"])
}

// TestHandlerManager_GetHandlerNames 测试获取处理器名称
func TestHandlerManager_GetHandlerNames(t *testing.T) {
	services := model.NewServices()
	manager := NewHandlerManager(services)

	names := manager.GetHandlerNames()

	expectedNames := []string{
		"数据解析器",
		"产品数据验证",
		"获取店铺信息",
		"检查每日上架限额",
		"产品类型处理器",
		"LLM属性映射器",
		"获取产品数据",
		"图片处理器",
		"变体处理器",
		"创建Amazon Listing",
		"设置产品价格",
		"库存处理器",
	}

	assert.Equal(t, expectedNames, names)
}

type failingHandler struct {
	name  string
	stage string
	err   error
}

func (h failingHandler) Name() string  { return h.name }
func (h failingHandler) Stage() string { return h.stage }
func (h failingHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	return h.err
}

func TestHandlerManager_ProcessProductWrapsStructuredTaskError(t *testing.T) {
	manager := &HandlerManager{
		services: model.NewServices(),
		logger:   logger.GetGlobalLogger("amazon/pipeline-test"),
		handlers: []Handler{
			failingHandler{
				name:  "创建Amazon Listing",
				stage: amazonTaskStageCreateListing,
				err:   model.NewAmazonError(model.ErrorCodeRateLimit, "slow down"),
			},
		},
	}

	taskContext := &model.TaskContext{
		TaskID: "test-wrap-001",
		Data:   map[string]any{"product_id": "demo"},
	}

	err := manager.ProcessProduct(context.Background(), taskContext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "[stage:"+amazonTaskStageCreateListing+"]")
	assert.Contains(t, err.Error(), "["+amazonTaskReasonRateLimit+"]")
	assert.NotContains(t, err.Error(), "NONRETRYABLE:")
}
