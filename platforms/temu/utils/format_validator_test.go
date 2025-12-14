package utils

import (
	"testing"
)

func TestFormatWeight(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: "0.22",
		},
		{
			name:     "正常重量",
			input:    "1.5",
			expected: "1.50",
		},
		{
			name:     "带单位的重量",
			input:    "2.345 lb",
			expected: "2.35",
		},
		{
			name:     "超过两位小数",
			input:    "3.14159",
			expected: "3.14",
		},
		{
			name:     "零重量",
			input:    "0",
			expected: "0.22",
		},
		{
			name:     "负重量",
			input:    "-1.5",
			expected: "0.22",
		},
		{
			name:     "超大重量",
			input:    "1000.5",
			expected: "999.99",
		},
		{
			name:     "无效格式",
			input:    "abc",
			expected: "0.22",
		},
		{
			name:     "中文单位",
			input:    "2.5磅",
			expected: "2.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatWeight(tt.input)
			if result != tt.expected {
				t.Errorf("FormatWeight(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatDimension(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: "3.9",
		},
		{
			name:     "正常尺寸",
			input:    "10.5",
			expected: "10.5",
		},
		{
			name:     "带单位的尺寸",
			input:    "15.67 in",
			expected: "15.7",
		},
		{
			name:     "超过一位小数",
			input:    "8.999",
			expected: "9.0",
		},
		{
			name:     "零尺寸",
			input:    "0",
			expected: "3.9",
		},
		{
			name:     "负尺寸",
			input:    "-5.2",
			expected: "3.9",
		},
		{
			name:     "超大尺寸",
			input:    "10000.5",
			expected: "9999.9",
		},
		{
			name:     "无效格式",
			input:    "xyz",
			expected: "3.9",
		},
		{
			name:     "中文单位",
			input:    "12.3英寸",
			expected: "12.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDimension(tt.input)
			if result != tt.expected {
				t.Errorf("FormatDimension(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}
