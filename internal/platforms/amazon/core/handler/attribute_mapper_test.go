// Package handler 提供属性映射处理器测试
package handler

import (
	"context"
	"task-processor/internal/platforms/amazon/core/model"
	"task-processor/internal/platforms/amazon/core/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestAttributeMapperHandler_Handle_WithLLM 测试使用LLM的属性映射
func TestAttributeMapperHandler_Handle_WithLLM(t *testing.T) {
	// 创建模拟LLM客户端
	mockLLMClient := &MockLLMClient{}

	// 模拟LLM响应
	mockResponse := &service.ChatResponse{
		Content: `{
			"mapped_attributes": {
				"item_name": "Elegant Korean Style Dress",
				"brand": "Fashion Brand",
				"product_description": "Beautiful Korean style dress with modern design",
				"color_name": "Black",
				"size_name": "Medium"
			},
			"product_type": "APPAREL",
			"confidence": 0.92,
			"reasoning": "Successfully mapped fashion product with high confidence"
		}`,
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     120,
			CompletionTokens: 60,
			TotalTokens:      180,
		},
	}

	mockLLMClient.On("Chat", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// 创建服务容器
	services := model.NewServices()
	llmMapper := service.NewLLMAttributeMapper(mockLLMClient)
	services.SetLLMAttributeMapper(llmMapper)

	// 创建处理器
	handler := NewAttributeMapperHandler(services)

	// 创建任务上下文
	taskContext := &model.TaskContext{
		TaskID:        "test-llm-mapping",
		MarketplaceID: "ATVPDKIKX0DER",
		Data: map[string]interface{}{
			"product_id": "test-123",
		},
	}

	// 设置原始产品数据
	productData := map[string]interface{}{
		"title":       "韩版修身显瘦连衣裙女装",
		"brand":       "时尚品牌",
		"description": "优雅的韩版修身连衣裙，显瘦效果好",
		"color":       "黑色",
		"size":        "M",
		"price":       "199.00",
	}
	taskContext.SetResult("raw_product_data", productData)

	// 执行处理
	ctx := context.Background()
	err := handler.Handle(ctx, taskContext)

	// 验证结果
	assert.NoError(t, err)

	// 检查映射结果
	mappedAttrs, exists := taskContext.GetResult("mapped_attributes")
	assert.True(t, exists)
	assert.NotNil(t, mappedAttrs)

	attributes := mappedAttrs.(map[string]interface{})
	assert.Equal(t, "Elegant Korean Style Dress", attributes["item_name"])
	assert.Equal(t, "Fashion Brand", attributes["brand"])

	// 检查产品类型
	productType, exists := taskContext.GetResult("product_type")
	assert.True(t, exists)
	assert.Equal(t, "APPAREL", productType)

	// 检查置信度
	confidence, exists := taskContext.GetResult("mapping_confidence")
	assert.True(t, exists)
	assert.Equal(t, 0.92, confidence)

	// 验证模拟调用
	mockLLMClient.AssertExpectations(t)
}

// TestAttributeMapperHandler_Handle_FallbackToBasic 测试回退到基础映射
func TestAttributeMapperHandler_Handle_FallbackToBasic(t *testing.T) {
	// 创建没有LLM服务的服务容器
	services := model.NewServices()

	// 创建处理器
	handler := NewAttributeMapperHandler(services)

	// 创建任务上下文
	taskContext := &model.TaskContext{
		TaskID: "test-basic-mapping",
		Data: map[string]interface{}{
			"product_id": "test-456",
		},
	}

	// 设置原始产品数据
	productData := map[string]interface{}{
		"title":       "测试产品标题",
		"brand":       "测试品牌",
		"description": "测试产品描述",
	}
	taskContext.SetResult("raw_product_data", productData)

	// 执行处理
	ctx := context.Background()
	err := handler.Handle(ctx, taskContext)

	// 验证结果
	assert.NoError(t, err)

	// 检查基础映射结果
	mappedAttrs, exists := taskContext.GetResult("mapped_attributes")
	assert.True(t, exists)
	assert.NotNil(t, mappedAttrs)

	attributes := mappedAttrs.(map[string]interface{})
	assert.Equal(t, "测试产品标题", attributes["item_name"])
	assert.Equal(t, "测试品牌", attributes["brand"])

	// 检查置信度（基础映射应该较低）
	confidence, exists := taskContext.GetResult("mapping_confidence")
	assert.True(t, exists)
	assert.Equal(t, 0.6, confidence)
}

// TestAttributeMapperHandler_Handle_MissingData 测试缺少数据的情况
func TestAttributeMapperHandler_Handle_MissingData(t *testing.T) {
	services := model.NewServices()
	handler := NewAttributeMapperHandler(services)

	taskContext := &model.TaskContext{
		TaskID: "test-missing-data",
		Data:   map[string]interface{}{},
	}

	// 不设置raw_product_data

	ctx := context.Background()
	err := handler.Handle(ctx, taskContext)

	// 应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "原始产品数据不存在")
}

// MockLLMClient 模拟LLM客户端（重用service包中的定义）
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) Chat(ctx context.Context, messages []service.ChatMessage) (*service.ChatResponse, error) {
	args := m.Called(ctx, messages)
	return args.Get(0).(*service.ChatResponse), args.Error(1)
}
