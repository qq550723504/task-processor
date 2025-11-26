package utils

import (
	"testing"
)

func TestCleanProductTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove Chinese characters",
			input:    "Product Name 产品名称 Test",
			expected: "Product Name Test",
		},
		{
			name:     "Remove space before comma",
			input:    "Color : Wit , Size : 40",
			expected: "Color : Wit, Size : 40",
		},
		{
			name:     "Complex case from logs",
			input:    "Over Door Hooks, Over Door Hanger, Over Door Hook Hanger 6 Hooks, Assembly-Free Matte Texture Bathroom For Clothes Towels Coat Robe Bag. ( Color : Wit , Size : 4022cm(1pcs) ( Color : Grijs , Size : 40",
			expected: "Over Door Hooks, Over Door Hanger, Over Door Hook Hanger 6 Hooks, Assembly-Free Matte Texture Bathroom For Clothes Towels Coat Robe Bag. ( Color : Wit, Size : 4022cm(1pcs) ( Color : Grijs, Size : 40",
		},
		{
			name:     "Multiple spaces before comma",
			input:    "Test  ,  Another  ,  Third",
			expected: "Test, Another, Third",
		},
		{
			name:     "Emoji removal",
			input:    "Product 😀 Name 🎉",
			expected: "Product Name",
		},
		{
			name:     "Mixed Chinese and English",
			input:    "颜色：红色 Color: Red",
			expected: "Color: Red",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanProductTitle(tt.input)
			if result != tt.expected {
				t.Errorf("CleanProductTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}
