// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestValidateDropRate(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	tests := []struct {
		name          string
		dropRate      int
		originalValue float64
		expected      int
	}{
		{
			name:          "正常范围内的值",
			dropRate:      50,
			originalValue: 0.5,
			expected:      50,
		},
		{
			name:          "小于1的值应该调整为1",
			dropRate:      0,
			originalValue: 0.0,
			expected:      1,
		},
		{
			name:          "负数应该调整为1",
			dropRate:      -5,
			originalValue: -0.05,
			expected:      1,
		},
		{
			name:          "等于100的值应该调整为99",
			dropRate:      100,
			originalValue: 1.0,
			expected:      99,
		},
		{
			name:          "大于100的值应该调整为99",
			dropRate:      150,
			originalValue: 1.5,
			expected:      99,
		},
		{
			name:          "边界值1应该保持不变",
			dropRate:      1,
			originalValue: 0.01,
			expected:      1,
		},
		{
			name:          "边界值99应该保持不变",
			dropRate:      99,
			originalValue: 0.99,
			expected:      99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateDropRate(tt.dropRate, tt.originalValue, logger)
			if result != tt.expected {
				t.Errorf("ValidateDropRate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateDropRateFromDiscount(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())

	tests := []struct {
		name         string
		discountRate float64
		expected     int
	}{
		{
			name:         "正常折扣率10%",
			discountRate: 0.1,
			expected:     10,
		},
		{
			name:         "正常折扣率50%",
			discountRate: 0.5,
			expected:     50,
		},
		{
			name:         "零折扣率应该使用默认值10%",
			discountRate: 0.0,
			expected:     10,
		},
		{
			name:         "负折扣率应该使用默认值10%",
			discountRate: -0.1,
			expected:     10,
		},
		{
			name:         "100%折扣率应该使用默认值10%",
			discountRate: 1.0,
			expected:     10,
		},
		{
			name:         "超过100%的折扣率应该使用默认值10%",
			discountRate: 1.5,
			expected:     10,
		},
		{
			name:         "边界值99%应该返回99",
			discountRate: 0.99,
			expected:     99,
		},
		{
			name:         "边界值1%应该返回1",
			discountRate: 0.01,
			expected:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDropRateFromDiscount(tt.discountRate, logger)
			if result != tt.expected {
				t.Errorf("CalculateDropRateFromDiscount() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
