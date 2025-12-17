package modules

import (
	"strings"
	"testing"
)

func TestValidateAndTruncateDescription(t *testing.T) {
	handler := &TranslateHandler{}

	tests := []struct {
		name        string
		input       string
		expectLen   int
		expectTrunc bool
	}{
		{
			name:        "短描述不截断",
			input:       "This is a short description.",
			expectLen:   28,
			expectTrunc: false,
		},
		{
			name:        "正好5000字符不截断",
			input:       strings.Repeat("a", 5000),
			expectLen:   5000,
			expectTrunc: false,
		},
		{
			name:        "超过5000字符需要截断",
			input:       strings.Repeat("a", 6000),
			expectLen:   5000,
			expectTrunc: true,
		},
		{
			name:        "超长描述在句号处截断",
			input:       strings.Repeat("a", 4900) + ". This is the end.",
			expectLen:   4918, // 不需要截断，总长度小于5000
			expectTrunc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateAndTruncateDescription(tt.input)

			if len(result) > 5000 {
				t.Errorf("结果长度 %d 超过了5000字符限制", len(result))
			}

			if tt.expectTrunc && len(result) >= len(tt.input) {
				t.Errorf("期望截断但未截断，输入长度: %d, 输出长度: %d", len(tt.input), len(result))
			}

			if !tt.expectTrunc && len(result) != tt.expectLen {
				t.Errorf("期望长度 %d, 实际长度 %d", tt.expectLen, len(result))
			}
		})
	}
}

func TestValidateAndTruncateDescriptionWithSentences(t *testing.T) {
	handler := &TranslateHandler{}

	// 创建一个超过5000字符的描述，包含多个句子
	longDesc := strings.Repeat("This is a test sentence. ", 250) // 约6250字符

	result := handler.validateAndTruncateDescription(longDesc)

	// 验证结果不超过5000字符
	if len(result) > 5000 {
		t.Errorf("结果长度 %d 超过了5000字符限制", len(result))
	}

	// 验证结果以句号结尾（因为应该在句子边界截断）
	if !strings.HasSuffix(result, ".") {
		t.Errorf("期望结果以句号结尾，但实际结尾是: %s", result[len(result)-10:])
	}

	t.Logf("原始长度: %d, 截断后长度: %d", len(longDesc), len(result))
}
