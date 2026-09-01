package strx

import "testing"

func TestContainsIgnoreCaseLegacy(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"测试中文", "中文", true},
	}

	for _, tt := range tests {
		result := ContainsIgnoreCase(tt.s, tt.substr)
		if result != tt.expected {
			t.Errorf("ContainsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
		}
	}
}

func TestFindSubstringLegacy(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "World", true},
		{"Hello World", "xyz", false},
		{"Hello World", "", true},
		{"", "test", false},
	}

	for _, tt := range tests {
		result := FindSubstring(tt.s, tt.substr)
		if result != tt.expected {
			t.Errorf("FindSubstring(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
		}
	}
}

func TestTruncateStringLegacy(t *testing.T) {
	tests := []struct {
		s        string
		maxLen   int
		expected string
	}{
		{"Hello World", 5, "Hello"},
		{"Hello", 10, "Hello"},
		{"测试", 1, "测"},
	}

	for _, tt := range tests {
		result := TruncateString(tt.s, tt.maxLen)
		if result != tt.expected {
			t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, result, tt.expected)
		}
	}
}
