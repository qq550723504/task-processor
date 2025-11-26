package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImagePaddingProcessor_CalculatePaddingDimensions(t *testing.T) {
	processor := NewImagePaddingProcessor()

	tests := []struct {
		name            string
		width           int
		height          int
		targetRatio     float64
		minWidth        int
		minHeight       int
		expectedWidth   int
		expectedHeight  int
		expectedPadding bool
	}{
		{
			name:            "正方形图片转3:4（服装类）",
			width:           1000,
			height:          1000,
			targetRatio:     0.75, // 3:4
			minWidth:        1340,
			minHeight:       1785,
			expectedWidth:   1340,
			expectedHeight:  1786, // 1340 / 0.75 = 1786.67，向下取整为1786
			expectedPadding: true,
		},
		{
			name:            "宽图片转1:1（通用类）",
			width:           1200,
			height:          800,
			targetRatio:     1.0, // 1:1
			minWidth:        800,
			minHeight:       800,
			expectedWidth:   1200,
			expectedHeight:  1200,
			expectedPadding: true,
		},
		{
			name:            "高图片转1:1（通用类）",
			width:           800,
			height:          1200,
			targetRatio:     1.0, // 1:1
			minWidth:        800,
			minHeight:       800,
			expectedWidth:   1200,
			expectedHeight:  1200,
			expectedPadding: true,
		},
		{
			name:            "已符合要求的图片",
			width:           1000,
			height:          1000,
			targetRatio:     1.0,
			minWidth:        800,
			minHeight:       800,
			expectedWidth:   1000,
			expectedHeight:  1000,
			expectedPadding: false,
		},
		{
			name:            "小尺寸图片需要放大",
			width:           600,
			height:          600,
			targetRatio:     1.0,
			minWidth:        800,
			minHeight:       800,
			expectedWidth:   800,
			expectedHeight:  800,
			expectedPadding: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newWidth, newHeight, needsPadding := processor.CalculatePaddingDimensions(
				tt.width,
				tt.height,
				tt.targetRatio,
				tt.minWidth,
				tt.minHeight,
			)

			assert.Equal(t, tt.expectedWidth, newWidth, "宽度不匹配")
			assert.Equal(t, tt.expectedHeight, newHeight, "高度不匹配")
			assert.Equal(t, tt.expectedPadding, needsPadding, "填充标志不匹配")

			// 验证宽高比
			if needsPadding {
				actualRatio := float64(newWidth) / float64(newHeight)
				tolerance := 0.01
				assert.InDelta(t, tt.targetRatio, actualRatio, tolerance, "宽高比不符合要求")
			}
		})
	}
}

func TestImagePaddingProcessor_AspectRatioCalculation(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		targetRatio float64
		description string
	}{
		{
			name:        "3:4宽高比（服装类）",
			width:       1340,
			height:      1785,
			targetRatio: 0.75,
			description: "服装类产品标准尺寸",
		},
		{
			name:        "1:1宽高比（通用类）",
			width:       800,
			height:      800,
			targetRatio: 1.0,
			description: "通用产品标准尺寸",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualRatio := float64(tt.width) / float64(tt.height)
			tolerance := 0.01
			assert.InDelta(t, tt.targetRatio, actualRatio, tolerance,
				"%s: 宽高比应为%.2f", tt.description, tt.targetRatio)
		})
	}
}
