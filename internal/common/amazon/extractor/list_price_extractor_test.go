// Package extractor 提供Amazon原价提取功能测试
package extractor

import (
	"task-processor/internal/common/amazon/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPriceExtractor_validatePriorityPrice(t *testing.T) {
	extractor := NewListPriceExtractor("US")

	tests := []struct {
		name          string
		text          string
		expectedValid bool
		expectedPrice string
	}{
		{
			name:          "典型价格格式",
			text:          "Typical price: $78.19",
			expectedValid: true,
			expectedPrice: "$78.19",
		},
		{
			name:          "List Price格式",
			text:          "List Price: $99.99",
			expectedValid: true,
			expectedPrice: "$99.99",
		},
		{
			name:          "无原价标识",
			text:          "$19.99",
			expectedValid: false,
			expectedPrice: "",
		},
		{
			name:          "空文本",
			text:          "",
			expectedValid: false,
			expectedPrice: "",
		},
		{
			name:          "包含其他文本的典型价格",
			text:          "Save 25% Typical price: $156.38",
			expectedValid: true,
			expectedPrice: "$156.38",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, price := extractor.validatePriorityPrice(tt.text)
			assert.Equal(t, tt.expectedValid, valid)
			assert.Equal(t, tt.expectedPrice, price)
		})
	}
}

func TestListPriceExtractor_PriceBreakdownAssignment(t *testing.T) {
	extractor := NewListPriceExtractor("US")

	// 测试价格解析和赋值逻辑
	product := &model.Product{
		FinalPrice:      19.99,
		PricesBreakdown: model.PriceBreakdown{},
	}

	// 模拟找到有效原价的情况
	valid, priceText := extractor.validatePriorityPrice("Typical price: $78.19")
	assert.True(t, valid)
	assert.Equal(t, "$78.19", priceText)

	// 解析价格
	listPrice := extractor.parser.ParsePrice(priceText)
	assert.Equal(t, 78.19, listPrice)

	// 验证价格赋值条件
	if listPrice > 0 && listPrice != product.FinalPrice && listPrice > product.FinalPrice {
		product.PricesBreakdown.ListPrice = &listPrice
	}

	assert.NotNil(t, product.PricesBreakdown.ListPrice)
	assert.Equal(t, 78.19, *product.PricesBreakdown.ListPrice)
}

func TestListPriceExtractor_NoListPriceScenario(t *testing.T) {
	extractor := NewListPriceExtractor("US")

	// 测试没有原价的情况
	product := &model.Product{
		FinalPrice:      19.99,
		PricesBreakdown: model.PriceBreakdown{},
	}

	// 模拟只有普通价格文本，没有原价标识
	valid, priceText := extractor.validatePriorityPrice("$19.99")
	assert.False(t, valid)
	assert.Equal(t, "", priceText)

	// 确保没有设置原价
	assert.Nil(t, product.PricesBreakdown.ListPrice)
}

func TestListPriceExtractor_containsOtherASIN(t *testing.T) {
	extractor := NewListPriceExtractor("US")

	tests := []struct {
		name        string
		value       string
		currentASIN string
		expected    bool
	}{
		{
			name:        "包含其他ASIN",
			value:       "data-asin-B0B9SK369P",
			currentASIN: "B0DL5PJG6D",
			expected:    true,
		},
		{
			name:        "包含当前ASIN",
			value:       "data-asin-B0DL5PJG6D",
			currentASIN: "B0DL5PJG6D",
			expected:    false,
		},
		{
			name:        "不包含ASIN",
			value:       "some-other-data",
			currentASIN: "B0DL5PJG6D",
			expected:    false,
		},
		{
			name:        "包含多个ASIN其中一个是其他的",
			value:       "B0DL5PJG6D-B0B9SK369P",
			currentASIN: "B0DL5PJG6D",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.containsOtherASIN(tt.value, tt.currentASIN)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestListPriceExtractor_containsOtherProductURL(t *testing.T) {
	extractor := NewListPriceExtractor("US")

	tests := []struct {
		name        string
		url         string
		currentASIN string
		expected    bool
	}{
		{
			name:        "指向其他产品的dp URL",
			url:         "https://www.amazon.com/dp/B0B9SK369P",
			currentASIN: "B0DL5PJG6D",
			expected:    true,
		},
		{
			name:        "指向当前产品的dp URL",
			url:         "https://www.amazon.com/dp/B0DL5PJG6D",
			currentASIN: "B0DL5PJG6D",
			expected:    false,
		},
		{
			name:        "指向其他产品的gp URL",
			url:         "https://www.amazon.com/gp/product/B0B9SK369P",
			currentASIN: "B0DL5PJG6D",
			expected:    true,
		},
		{
			name:        "不包含产品URL",
			url:         "https://www.amazon.com/some-other-page",
			currentASIN: "B0DL5PJG6D",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.containsOtherProductURL(tt.url, tt.currentASIN)
			assert.Equal(t, tt.expected, result)
		})
	}
}
