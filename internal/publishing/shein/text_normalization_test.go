package shein

import (
	"strings"
	"testing"
)

func TestSanitizeSheinAttributeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"spaces", "   ", "Custom Value"},
		{"special only", "!!!###", "Custom Value"},
		{"normal text", "Red Color", "Red Color"},
		{"comma to space", "Red,Blue", "Red Blue"},
		{"inch", `12"`, "12 inch"},
		{"feet", `5'`, "5 ft"},
		{"dimension", "10 x 20", "10 by 20"},
		{"asterisk dimension", "11.8*11.8 IN", "11.8 by 11.8 IN"},
		{"and", "Black & White", "Black and White"},
		{"percent", "50%", "50 percent"},
		{"chinese punctuation", "颜色（红色）", "颜色 红色"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeSheinAttributeText(tt.input); got != tt.want {
				t.Fatalf("sanitizeSheinAttributeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidSheinAttributeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", false},
		{"spaces", "   ", false},
		{"normal", "Red", true},
		{"special", "Red,Blue", false},
		{"long", strings.Repeat("a", 101), false},
		{"max", strings.Repeat("a", 100), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidSheinAttributeText(tt.input); got != tt.want {
				t.Fatalf("isValidSheinAttributeText(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
