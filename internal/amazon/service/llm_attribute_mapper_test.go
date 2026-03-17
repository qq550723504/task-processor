// Package service 提供LLM属性映射器测试
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLLMClient 模拟LLM客户端
type MockLLMClient struct {
	mock.Mock
}

// Chat 模拟聊天方法
func (m *MockLLMClient) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	args := m.Called(ctx, messages)
	return args.Get(0).(*ChatResponse), args.Error(1)
}

// TestLLMAttributeMapper_MapAttributes 测试LLM属性映射
func TestLLMAttributeMapper_MapAttributes(t *testing.T) {
	// 创建模拟客户端
	mockClient := new(MockLLMClient)

	// 模拟LLM响应
	mockResponse := &ChatResponse{
		Content: `{
			"mapped_attributes": {
				"item_name": "Korean Style Slim Fit Long Sleeve Dress",
				"brand": "Fashion Brand",
				"product_description": "Elegant Korean style dress with slim fit design",
				"color_name": "Black",
				"material_type": "Cotton Blend"
			},
			"product_type": "APPAREL",
			"confidence": 0.95,
			"reasoning": "Successfully mapped Chinese product to Amazon format with high confidence"
		}`,
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     150,
			CompletionTokens: 80,
			TotalTokens:      230,
		},
	}

	// 设置模拟期望
	mockClient.On("Chat", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// 创建LLM映射器
	mapper := NewLLMAttributeMapper(mockClient)

	// 准备测试数据
	req := &AttributeMappingRequest{
		SourcePlatform: "1688",
		TargetPlatform: "Amazon",
		ProductType:    "APPAREL",
		ProductData: map[string]any{
			"title":       "韩版修身显瘦长袖连衣裙",
			"brand":       "时尚品牌",
			"description": "优雅的韩版修身连衣裙，显瘦设计",
			"color":       "黑色",
			"material":    "棉混纺",
		},
	}

	// 执行映射
	ctx := context.Background()
	resp, err := mapper.MapAttributes(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "APPAREL", resp.ProductType)
	assert.Equal(t, 0.95, resp.Confidence)
	assert.Contains(t, resp.MappedAttributes, "item_name")
	assert.Equal(t, "Korean Style Slim Fit Long Sleeve Dress", resp.MappedAttributes["item_name"])

	// 验证模拟调用
	mockClient.AssertExpectations(t)
}

// TestLLMAttributeMapper_ValidateMapping 测试映射验证
func TestLLMAttributeMapper_ValidateMapping(t *testing.T) {
	mapper := NewLLMAttributeMapper(nil)

	// 测试有效映射
	validAttributes := map[string]any{
		"item_name": "Test Product",
		"brand":     "Test Brand",
	}
	err := mapper.ValidateMapping(validAttributes)
	assert.NoError(t, err)

	// 测试缺少必需字段
	invalidAttributes := map[string]any{
		"brand": "Test Brand",
	}
	err = mapper.ValidateMapping(invalidAttributes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少必需属性: item_name")

	// 测试空标题
	emptyTitleAttributes := map[string]any{
		"item_name": "",
	}
	err = mapper.ValidateMapping(emptyTitleAttributes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "item_name不能为空")
}

// TestLLMAttributeMapper_ParseLLMResponse 测试响应解析
func TestLLMAttributeMapper_ParseLLMResponse(t *testing.T) {
	mapper := NewLLMAttributeMapper(nil)

	// 测试有效JSON响应
	validJSON := `{
		"mapped_attributes": {
			"item_name": "Test Product"
		},
		"product_type": "PRODUCT",
		"confidence": 0.8,
		"reasoning": "Test reasoning"
	}`

	result, err := mapper.parseLLMResponse(validJSON)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "PRODUCT", result.ProductType)
	assert.Equal(t, 0.8, result.Confidence)

	// 测试无效JSON
	invalidJSON := "invalid json"
	result, err = mapper.parseLLMResponse(invalidJSON)
	assert.Error(t, err)
	assert.Nil(t, result)

	// 测试缺少必要字段的JSON
	incompleteJSON := `{"product_type": "PRODUCT"}`
	result, err = mapper.parseLLMResponse(incompleteJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "映射结果缺少mapped_attributes字段")
}

// TestLLMAttributeMapper_GetMappingStats 测试获取统计信息
func TestLLMAttributeMapper_GetMappingStats(t *testing.T) {
	mapper := NewLLMAttributeMapper(nil)

	stats := mapper.GetMappingStats()

	assert.NotNil(t, stats)
	assert.Equal(t, "LLMAttributeMapper", stats["service_name"])
	assert.Equal(t, "1.0.0", stats["version"])
	assert.Contains(t, stats["supported_pairs"], "1688->Amazon")
}
