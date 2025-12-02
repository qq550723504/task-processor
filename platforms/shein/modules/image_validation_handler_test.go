package modules

import (
	"task-processor/common/amazon"
	"testing"
)

func TestImageValidationHandler_Handle(t *testing.T) {
	tests := []struct {
		name          string
		images        []string
		minImageCount int
		wantErr       bool
		errType       string
	}{
		{
			name:          "图片数量充足",
			images:        []string{"img1.jpg", "img2.jpg", "img3.jpg", "img4.jpg"},
			minImageCount: 3,
			wantErr:       false,
		},
		{
			name:          "图片数量刚好满足要求",
			images:        []string{"img1.jpg", "img2.jpg", "img3.jpg"},
			minImageCount: 3,
			wantErr:       false,
		},
		{
			name:          "图片数量不足",
			images:        []string{"img1.jpg", "img2.jpg"},
			minImageCount: 3,
			wantErr:       true,
			errType:       "NonRetryableError",
		},
		{
			name:          "只有一张图片",
			images:        []string{"img1.jpg"},
			minImageCount: 3,
			wantErr:       true,
			errType:       "NonRetryableError",
		},
		{
			name:          "没有图片",
			images:        []string{},
			minImageCount: 3,
			wantErr:       true,
			errType:       "NonRetryableError",
		},
		{
			name:          "包含空字符串的图片",
			images:        []string{"img1.jpg", "", "img2.jpg", "img3.jpg"},
			minImageCount: 3,
			wantErr:       false,
		},
		{
			name:          "包含空字符串导致数量不足",
			images:        []string{"img1.jpg", "", "img2.jpg"},
			minImageCount: 3,
			wantErr:       true,
			errType:       "NonRetryableError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewImageValidationHandler(tt.minImageCount)

			ctx := &TaskContext{
				AmazonProduct: &amazon.Product{
					Asin:   "B001TEST",
					Images: tt.images,
				},
			}

			err := handler.Handle(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errType == "NonRetryableError" {
				// 验证错误是 RetryableError 接口且不可重试
				if retryErr, ok := err.(RetryableError); ok {
					if retryErr.IsRetryable() {
						t.Errorf("期望不可重试错误，但错误可重试")
					}
				} else {
					t.Errorf("期望 RetryableError 接口，但得到: %T", err)
				}
			}
		})
	}
}

func TestImageValidationHandler_NoProduct(t *testing.T) {
	handler := NewImageValidationHandler(3)
	ctx := &TaskContext{
		AmazonProduct: nil,
	}

	err := handler.Handle(ctx)
	if err == nil {
		t.Error("期望返回错误，但没有错误")
	}
}

func TestNewImageValidationHandler_DefaultMinCount(t *testing.T) {
	tests := []struct {
		name          string
		inputCount    int
		expectedCount int
	}{
		{
			name:          "使用指定的最小数量",
			inputCount:    5,
			expectedCount: 5,
		},
		{
			name:          "使用默认最小数量（输入0）",
			inputCount:    0,
			expectedCount: 3,
		},
		{
			name:          "使用默认最小数量（输入负数）",
			inputCount:    -1,
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewImageValidationHandler(tt.inputCount)
			if handler.minImageCount != tt.expectedCount {
				t.Errorf("minImageCount = %d, 期望 %d", handler.minImageCount, tt.expectedCount)
			}
		})
	}
}
