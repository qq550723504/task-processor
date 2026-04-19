package extractor

import (
	"testing"
)

func TestAvailabilityExtractor_IsAvailable(t *testing.T) {
	extractor := &AvailabilityExtractor{}

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "西班牙语 - Disponible",
			text:     "Disponible",
			expected: true,
		},
		{
			name:     "西班牙语 - No disponible",
			text:     "No disponible",
			expected: false,
		},
		{
			name:     "英语 - In Stock",
			text:     "In Stock",
			expected: true,
		},
		{
			name:     "英语 - Out of Stock",
			text:     "Out of Stock",
			expected: false,
		},
		{
			name:     "日语 - 在庫あり",
			text:     "在庫あり",
			expected: true,
		},
		{
			name:     "日语 - 在庫切れ",
			text:     "在庫切れ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.isAvailable(tt.text)
			if result != tt.expected {
				t.Errorf("isAvailable(%q) = %v, 期望 %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestAvailabilityExtractor_IsValidAvailabilityTextIgnoresImplementationNoise(t *testing.T) {
	extractor := &AvailabilityExtractor{}

	text := "// Nav start should be logged at this place only if request is NOT progressively loaded."
	if extractor.isValidAvailabilityText(text) {
		t.Fatalf("isValidAvailabilityText(%q) should be false", text)
	}
}

func TestContainsAvailabilityKeywordDoesNotMatchIdentifiers(t *testing.T) {
	if containsAvailabilityKeyword("window.$nav.declare('hamburgermenuiconavailableonload', false)", "available") {
		t.Fatal("available 不应匹配脚本标识符里的子串")
	}

	if !containsAvailabilityKeyword("currently available for delivery", "available") {
		t.Fatal("available 应匹配自然语言文本")
	}
}

func TestAvailabilityExtractor_IsBodyAvailabilityCandidate(t *testing.T) {
	extractor := &AvailabilityExtractor{}

	if extractor.isBodyAvailabilityCandidate("FREE delivery Friday, April 24") {
		t.Fatal("纯配送文案不应被当作库存文案")
	}

	if !extractor.isBodyAvailabilityCandidate("在庫あり") {
		t.Fatal("日文库存文案应被识别")
	}
}

func TestAvailabilityExtractor_PreorderTexts(t *testing.T) {
	extractor := &AvailabilityExtractor{}

	texts := []string{
		"预售商品：将于2026年4月22日有货，",
		"可预先订购。",
		"予約受付中",
	}

	for _, text := range texts {
		if !extractor.isValidAvailabilityText(text) {
			t.Fatalf("isValidAvailabilityText(%q) should be true", text)
		}

		if !extractor.isAvailable(text) {
			t.Fatalf("isAvailable(%q) should be true", text)
		}
	}
}
