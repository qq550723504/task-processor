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
