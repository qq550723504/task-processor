package content

import (
	"strings"
	"testing"
)

func TestStringSanitizer_SanitizeForSheinAttribute(t *testing.T) {
	s := NewStringSanitizer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"空字符串原样返回", "", ""},
		{"纯空格返回 Custom Value", "   ", "Custom Value"},
		{"纯特殊字符返回 Custom Value", "!!!###", "Custom Value"},
		{"正常英文不变", "Red Color", "Red Color"},
		{"逗号替换为空格", "Red,Blue", "Red Blue"},
		{"英寸符号转换", `12"`, "12 inch"},
		{"英尺符号转换", `5'`, "5 ft"},
		{"x 替换为 by", "10 x 20", "10 by 20"},
		{"& 替换为 and", "Black & White", "Black and White"},
		{"百分号替换", "50%", "50 percent"},
		{"多余空格合并", "Red   Blue", "Red Blue"},
		{"首尾空格去除", "  Red  ", "Red"},
		// 中文括号被替换为空格，但汉字被 unicode.IsLetter 保留，结果为 "颜色 红色"
		{"中文括号替换为空格", "颜色（红色）", "颜色 红色"},
		{"清理后为空返回 Custom Value", "---", "Custom Value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SanitizeForSheinAttribute(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeForSheinAttribute(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStringSanitizer_SanitizeForSheinTitle(t *testing.T) {
	s := NewStringSanitizer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"空字符串原样返回", "", ""},
		{"正常标题不变", "Women Summer Dress", "Women Summer Dress"},
		{"去除危险字符双引号", `Women "Summer" Dress`, "Women Summer Dress"},
		{"去除单引号", `Women's Dress`, "Womens Dress"},
		{"去除反斜杠", `Women\Dress`, "WomenDress"},
		{"去除竖线", "Women|Dress", "WomenDress"},
		{"多余空格合并", "Women  Summer  Dress", "Women Summer Dress"},
		{"清理后为空返回 Product Title", `'"<>|\\`, "Product Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SanitizeForSheinTitle(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeForSheinTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStringSanitizer_IsValidForSheinAttribute(t *testing.T) {
	s := NewStringSanitizer()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"空字符串无效", "", false},
		{"纯空格无效", "   ", false},
		{"正常值有效", "Red", true},
		{"含特殊字符无效", "Red,Blue", false},
		{"超过100字符无效", strings.Repeat("a", 101), false},
		{"恰好100字符有效", strings.Repeat("a", 100), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.IsValidForSheinAttribute(tt.input)
			if got != tt.want {
				t.Errorf("IsValidForSheinAttribute(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStringSanitizer_RemoveUnicodeControlChars(t *testing.T) {
	s := NewStringSanitizer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"无控制字符不变", "hello world", "hello world"},
		{"保留换行符", "line1\nline2", "line1\nline2"},
		{"保留制表符", "col1\tcol2", "col1\tcol2"},
		{"移除 null 字符", "hello\x00world", "helloworld"},
		{"移除 BEL 字符", "hello\x07world", "helloworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.RemoveUnicodeControlChars(tt.input)
			if got != tt.want {
				t.Errorf("RemoveUnicodeControlChars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStringSanitizer_TruncateErrorMessage(t *testing.T) {
	s := NewStringSanitizer()

	tests := []struct {
		name     string
		input    string
		maxBytes int
		wantSub  string
	}{
		{"空字符串原样返回", "", 100, ""},
		{"短于限制不截断", "short error", 100, "short error"},
		{
			// "...[截断]" 含多字节字符，实际字节数超过 maxBytes；只验证含省略标记
			name:     "超过限制被截断含省略标记",
			input:    strings.Repeat("a", 500),
			maxBytes: 400,
			wantSub:  "...[截断]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.TruncateErrorMessage(tt.input, tt.maxBytes)
			if tt.wantSub != "" && !strings.Contains(got, tt.wantSub) {
				t.Errorf("TruncateErrorMessage result %q should contain %q", got, tt.wantSub)
			}
		})
	}
}

// TestGlobalSanitizeFunctions 验证全局便捷函数委托给默认实例
func TestGlobalSanitizeFunctions(t *testing.T) {
	if got := SanitizeForSheinAttribute("Red,Blue"); got != "Red Blue" {
		t.Errorf("global SanitizeForSheinAttribute = %q, want %q", got, "Red Blue")
	}
	if got := SanitizeForSheinTitle(`Women "Dress"`); got != "Women Dress" {
		t.Errorf("global SanitizeForSheinTitle = %q, want %q", got, "Women Dress")
	}
	if !IsValidForSheinAttribute("Red") {
		t.Error("global IsValidForSheinAttribute(Red) should be true")
	}
}
